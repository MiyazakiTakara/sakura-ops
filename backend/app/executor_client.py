"""Fail-closed API client for the separate admission-only executor.

No subprocess is started by this module. Socket peer must be a different
non-root Unix UID. No SSH keys, repository contents or user commands sent.
"""
import json
import os
import socket
import stat
import struct
from pathlib import Path

from fastapi import HTTPException

from . import preflight

MAX_RESPONSE = 8192


def _path():
    raw = os.getenv("SAKURAOPS_EXECUTOR_SOCKET", "")
    if not raw or not Path(raw).is_absolute():
        raise HTTPException(409, "Oddzielny wykonawca nie jest skonfigurowany")
    path = Path(raw)
    try:
        info = path.lstat()
        parent = path.parent.lstat()
    except OSError as exc:
        raise HTTPException(503, "Usługa wykonawcza nie działa") from exc
    if not stat.S_ISSOCK(info.st_mode) or not stat.S_ISDIR(parent.st_mode):
        raise HTTPException(503, "Nieprawidłowe gniazdo wykonawcy")
    if (info.st_mode & 0o007 or parent.st_mode & 0o022 or
            parent.st_uid != info.st_uid):
        raise HTTPException(503, "Gniazdo wykonawcy ma niezabezpieczone uprawnienia")
    if info.st_uid in (0, os.geteuid()):
        raise HTTPException(503, "Wykonawca musi działać jako inny użytkownik bez roota")
    return path, info.st_uid


def exchange(message):
    path, expected_uid = _path()
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as client:
        client.settimeout(2.0)
        try:
            client.connect(str(path))
            creds = client.getsockopt(socket.SOL_SOCKET, socket.SO_PEERCRED, 12)
            _, uid, _ = struct.unpack("3i", creds)
            if uid != expected_uid or uid in (0, os.geteuid()):
                raise HTTPException(503, "Niepoprawna tożsamość wykonawcy")
            raw = json.dumps(message, sort_keys=True, separators=(",", ":")).encode() + b"\n"
            if len(raw) > 4096:
                raise HTTPException(422, "Zbyt duże zlecenie")
            client.sendall(raw)
            reply = bytearray()
            while not reply.endswith(b"\n"):
                chunk = client.recv(1024)
                if not chunk or len(reply) + len(chunk) > MAX_RESPONSE:
                    raise HTTPException(502, "Nieprawidłowa odpowiedź wykonawcy")
                reply.extend(chunk)
        except (OSError, TimeoutError) as exc:
            raise HTTPException(503, "Wykonawca nie odpowiada") from exc
    try:
        response = json.loads(reply)
    except (ValueError, UnicodeDecodeError) as exc:
        raise HTTPException(502, "Nieprawidłowy format odpowiedzi") from exc
    if not isinstance(response, dict):
        raise HTTPException(502, "Nieprawidłowa odpowiedź")
    if "error" in response:
        raise HTTPException(409, "Wykonawca odrzucił zlecenie: " + str(response["error"])[:200])
    if "ok" not in response:
        raise HTTPException(502, "Brak potwierdzenia wykonawcy")
    return response["ok"]


def health():
    return exchange({"op": "status"})


def submit(review_id):
    # Review is persisted and must have been explicitly approved by admin.
    row = preflight.get(review_id)
    if row["state"] != "approved":
        raise HTTPException(409, "Wymagany zatwierdzony preflight")
    report = row["report"]
    current, fingerprint = preflight._assess(
        report["repository"], report["inventory"], report["playbook"],
        report["group"], report["mode"])
    if not current["eligible"] or fingerprint != row["fingerprint"]:
        raise HTTPException(409, "Zatwierdzony preflight jest nieaktualny")
    # A dry admission receipt, not a remote task. No file or command is sent.
    response = exchange({"op": "submit", "sha": current["git_sha"],
                         "fingerprint": fingerprint, "hosts": current["hosts"]})
    return {"review_id": review_id, "receipt": response, "executable": False}


def read(receipt_id):
    return exchange({"op": "get", "id": receipt_id})


def cancel(receipt_id):
    return exchange({"op": "cancel", "id": receipt_id})

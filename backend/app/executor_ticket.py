"""Short-lived one-use, canonical HMAC authorization for executor admission.

This ticket authorizes only metadata validation. It must not grant execution.
Both processes receive separate 0600 copies of the same random key.
"""
import hashlib
import hmac
import json
import os
import re
import secrets
import stat
import time
from pathlib import Path

SHA = re.compile(r"[0-9a-f]{40}\Z")
HEX = re.compile(r"[0-9a-f]{64}\Z")
NONCE = re.compile(r"[0-9a-f]{32}\Z")
HOST = re.compile(r"[A-Za-z0-9][A-Za-z0-9_.:@-]{0,254}\Z")
FIELDS = {"version", "nonce", "issued", "expires", "sha", "fingerprint",
          "hosts", "mode", "profile", "actor"}
PROFILES = {"builtin-ping"}


def load_key():
    raw = os.getenv("SAKURAOPS_EXECUTOR_APPROVAL_KEY", "")
    if not raw or not Path(raw).is_absolute():
        raise ValueError("Approval key path is not configured")
    path = Path(raw)
    info = path.lstat()
    if (not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid()
            or info.st_mode & 0o077):
        raise ValueError("Approval key must be owner-only and owned by the process UID")
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0) | getattr(os, "O_CLOEXEC", 0)
    fd = os.open(path, flags)
    try:
        after = os.fstat(fd)
        if after.st_dev != info.st_dev or after.st_ino != info.st_ino:
            raise ValueError("Approval key changed during validation")
        key = os.read(fd, 65)
        if len(key) != 32:
            raise ValueError("Approval key must be exactly 32 random bytes")
        return key
    finally:
        os.close(fd)


def encode(fields):
    return json.dumps(fields, sort_keys=True, separators=(",", ":"), ensure_ascii=True).encode("ascii")


def sign(fields, key):
    return hmac.new(key, encode(fields), hashlib.sha256).hexdigest()


def validate_fields(fields, now=None):
    if not isinstance(fields, dict) or set(fields) != FIELDS:
        raise ValueError("Unknown signed ticket fields")
    if (type(fields["version"]) is not int or fields["version"] != 1
            or not isinstance(fields["nonce"], str) or not NONCE.fullmatch(fields["nonce"])
            or not isinstance(fields["sha"], str) or not SHA.fullmatch(fields["sha"])
            or not isinstance(fields["fingerprint"], str)
            or not HEX.fullmatch(fields["fingerprint"])):
        raise ValueError("Invalid signature context")
    issued, expires = fields["issued"], fields["expires"]
    current = int(time.time()) if now is None else now
    if (type(issued) is not int or type(expires) is not int or
            issued > current + 5 or issued < current - 60 or
            expires <= current or expires > issued + 60):
        raise ValueError("Ticket expired or outside bounded time window")
    hosts = fields["hosts"]
    if (not isinstance(hosts, list) or not 1 <= len(hosts) <= 20
            or any(not isinstance(host, str) or not HOST.fullmatch(host) for host in hosts)
            or hosts != sorted(set(hosts))):
        raise ValueError("Invalid signed target set")
    if (fields["mode"] not in ("diagnostic", "check")
            or fields["profile"] not in PROFILES
            or not isinstance(fields["actor"], str)
            or not re.fullmatch(r"[A-Za-z0-9_.@-]{1,100}", fields["actor"])):
        raise ValueError("Unknown diagnostic profile or approver")
    return fields


def issue(sha, fingerprint, hosts, mode, actor, key, now=None):
    current = int(time.time()) if now is None else now
    fields = {"version": 1, "nonce": secrets.token_hex(16), "issued": current,
              "expires": current + 30, "sha": sha, "fingerprint": fingerprint,
              "hosts": sorted(hosts), "mode": mode, "profile": "builtin-ping",
              "actor": actor}
    validate_fields(fields, now=current)
    return {**fields, "signature": sign(fields, key)}


def verify(ticket, key, now=None):
    if not isinstance(ticket, dict) or set(ticket) != FIELDS | {"signature"}:
        raise ValueError("Unknown ticket envelope")
    signature = ticket["signature"]
    if not isinstance(signature, str) or not HEX.fullmatch(signature):
        raise ValueError("Invalid signature")
    fields = {k: ticket[k] for k in FIELDS}
    validate_fields(fields, now=now)
    if not hmac.compare_digest(signature, sign(fields, key)):
        raise ValueError("Ticket signature mismatch")
    return fields

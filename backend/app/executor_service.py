"""Standalone Unix-socket executor admission service.

The only accepted job is a no-command dry admission receipt. There is no
Ansible invocation, network client, subprocess or dynamic Python evaluation.
Run this in a separate, locked-down OS identity, never inside uvicorn.
"""
import argparse
import json
import os
import re
import signal
import socket
import socketserver
import sqlite3
import stat
import struct
import threading
import time
from datetime import datetime, timezone
from contextlib import contextmanager
from pathlib import Path

MAX_REQUEST = 4096
MAX_ACTIVE = 10
SHA = re.compile(r"[0-9a-f]{40}\Z")
DIGEST = re.compile(r"[0-9a-f]{64}\Z")
HOST = re.compile(r"[A-Za-z0-9_.:@-]{1,255}\Z")


def timestamp():
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


class Store:
    def __init__(self, path):
        self.path = str(path)
        self.lock = threading.RLock()
        with self.connect() as db:
            db.execute("""CREATE TABLE IF NOT EXISTS receipts (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                fingerprint TEXT NOT NULL, sha TEXT NOT NULL,
                hosts TEXT NOT NULL, state TEXT NOT NULL,
                created_at TEXT NOT NULL, finished_at TEXT,
                client_uid INTEGER NOT NULL)""")

    @contextmanager
    def connect(self):
        db = sqlite3.connect(self.path, timeout=3)
        try:
            db.execute("PRAGMA busy_timeout=3000")
            with db:
                yield db
        finally:
            db.close()

    @staticmethod
    def dto(row):
        return {"id": row[0], "fingerprint": row[1], "sha": row[2],
                "hosts": json.loads(row[3]), "state": row[4],
                "created_at": row[5], "finished_at": row[6]}

    def submit(self, fingerprint, sha, hosts, uid):
        if (not isinstance(fingerprint, str) or not DIGEST.fullmatch(fingerprint)
                or not isinstance(sha, str) or not SHA.fullmatch(sha)
                or not isinstance(hosts, list) or not 1 <= len(hosts) <= 20
                or len(set(hosts)) != len(hosts)
                or any(not isinstance(h, str) or not HOST.fullmatch(h) for h in hosts)):
            raise ValueError("Invalid fixed admission envelope")
        with self.lock, self.connect() as db:
            active = db.execute("SELECT hosts FROM receipts WHERE state='queued'").fetchall()
            if len(active) >= MAX_ACTIVE:
                raise ValueError("Executor admission queue is full")
            for (encoded,) in active:
                if set(json.loads(encoded)) & set(hosts):
                    raise ValueError("Target is already locked by another pending receipt")
            cursor = db.execute("""INSERT INTO receipts
                (fingerprint, sha, hosts, state, created_at, client_uid)
                VALUES (?, ?, ?, 'queued', ?, ?)""",
                (fingerprint, sha, json.dumps(sorted(hosts)), timestamp(), uid))
            row_id = cursor.lastrowid
            return self.get(row_id, db)

    def get(self, receipt_id, db=None):
        if not isinstance(receipt_id, int) or isinstance(receipt_id, bool) or receipt_id < 1:
            raise ValueError("Invalid receipt ID")
        def read(conn):
            row = conn.execute("""SELECT id, fingerprint, sha, hosts, state,
                created_at, finished_at FROM receipts WHERE id=?""", (receipt_id,)).fetchone()
            if row is None:
                raise ValueError("Receipt not found")
            return self.dto(row)
        if db is not None:
            return read(db)
        with self.lock, self.connect() as connection:
            return read(connection)

    def cancel(self, receipt_id):
        with self.lock, self.connect() as db:
            item = self.get(receipt_id, db)
            if item["state"] == "queued":
                db.execute("UPDATE receipts SET state='cancelled', finished_at=? WHERE id=?",
                           (timestamp(), receipt_id))
            return self.get(receipt_id, db)

    def acknowledge(self, receipt_id):
        """Simulation only: immediately complete a receipt; never executes a command."""
        with self.lock, self.connect() as db:
            item = self.get(receipt_id, db)
            if item["state"] == "queued":
                db.execute("UPDATE receipts SET state='validated', finished_at=? WHERE id=?",
                           (timestamp(), receipt_id))
            return self.get(receipt_id, db)

    def process_pending(self):
        """Complete queued *admission checks*; executes no payload."""
        with self.lock, self.connect() as db:
            items = db.execute("SELECT id FROM receipts WHERE state='queued' ORDER BY id LIMIT 10").fetchall()
        for (receipt_id,) in items:
            self.acknowledge(receipt_id)

    def recover(self):
        with self.lock, self.connect() as db:
            db.execute("UPDATE receipts SET state='interrupted', finished_at=? WHERE state='queued'",
                       (timestamp(),))


class Protocol:
    def __init__(self, store, allowed_uid):
        self.store = store
        self.allowed_uid = allowed_uid

    def handle(self, message, uid):
        if uid != self.allowed_uid:
            raise PermissionError("Caller UID not permitted")
        if not isinstance(message, dict) or set(message) not in (
            {"op"}, {"op", "id"}, {"op", "fingerprint", "sha", "hosts"}):
            raise ValueError("Unknown request envelope")
        op = message.get("op")
        if op == "status" and set(message) == {"op"}:
            return {"version": 1, "mode": "admission_only", "exec_enabled": False}
        if op == "submit" and set(message) == {"op", "fingerprint", "sha", "hosts"}:
            return self.store.submit(message["fingerprint"], message["sha"],
                                     message["hosts"], uid)
        if op in ("get", "cancel") and set(message) == {"op", "id"}:
            if op == "get":
                return self.store.get(message["id"])
            return self.store.cancel(message["id"])
        raise ValueError("Unsupported operation")


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        creds = self.request.getsockopt(socket.SOL_SOCKET, socket.SO_PEERCRED, 12)
        pid, uid, gid = struct.unpack("3i", creds)
        if uid != self.server.protocol.allowed_uid:
            self.wfile.write(b'{"error":"Forbidden"}\n')
            return
        raw = self.rfile.readline(MAX_REQUEST + 1)
        if not raw or len(raw) > MAX_REQUEST or not raw.endswith(b"\n"):
            self.wfile.write(b'{"error":"Invalid frame"}\n')
            return
        try:
            message = json.loads(raw)
            result = self.server.protocol.handle(message, uid)
            response = {"ok": result}
        except (ValueError, TypeError, KeyError, json.JSONDecodeError) as exc:
            response = {"error": str(exc)}
        self.wfile.write((json.dumps(response, separators=(",", ":")) + "\n").encode())


class Server(socketserver.ThreadingUnixStreamServer):
    daemon_threads = True
    allow_reuse_address = False

    def __init__(self, path, protocol):
        self.protocol = protocol
        super().__init__(path, Handler)


def serve(socket_path, database, api_uid):
    sock = Path(socket_path)
    db = Path(database)
    for path in (sock, db):
        if not path.is_absolute() or path.is_symlink():
            raise ValueError("Use absolute non-symlink paths")
    if not sock.parent.is_dir() or not db.parent.is_dir():
        raise ValueError("Directories must be provisioned outside the application")
    if sock.exists() or sock.is_symlink():
        raise ValueError("Socket path exists; remove stale socket only after verifying ownership")
    if os.geteuid() == api_uid or os.geteuid() == 0:
        raise ValueError("Executor must be a distinct non-root OS user")
    old_umask = os.umask(0o077)
    try:
        store = Store(db)
        os.chmod(db, 0o600)
        store.recover()
        with Server(str(sock), Protocol(store, api_uid)) as server:
            os.chmod(sock, 0o660)
            stop = threading.Event()
            def drain():
                while not stop.wait(0.5):
                    store.process_pending()
            worker = threading.Thread(target=drain, name="admission-only", daemon=True)
            worker.start()
            try:
                server.serve_forever(poll_interval=0.2)
            finally:
                stop.set()
                worker.join(timeout=2)
                sock.unlink(missing_ok=True)
    finally:
        os.umask(old_umask)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--socket", required=True)
    parser.add_argument("--database", required=True)
    parser.add_argument("--api-uid", required=True, type=int)
    args = parser.parse_args()
    serve(args.socket, args.database, args.api_uid)


if __name__ == "__main__":
    main()

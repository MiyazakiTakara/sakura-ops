"""Signed approval tickets, replay prevention and Unix socket security."""
import json
import os
import socket
import threading

import pytest
from fastapi import HTTPException
from backend.app import executor_client, executor_service, executor_ticket as tickets


KEY = b"K" * 32


@pytest.fixture
def store(tmp_path):
    return executor_service.Store(tmp_path / "receipts.sqlite")


def ticket(hosts=None, now=None):
    return tickets.issue("a" * 40, "b" * 64,
                         ["semaphore"] if hosts is None else hosts,
                         "diagnostic", "admin", KEY, now=now)


def envelope(hosts=None):
    return {"op": "submit", "ticket": ticket(hosts)}


def test_only_narrow_protocol_and_authorized_uid(store):
    protocol = executor_service.Protocol(store, 1001, KEY)
    with pytest.raises(PermissionError):
        protocol.handle({"op": "status"}, 1002)
    assert protocol.handle({"op": "status"}, 1001) == {
        "version": 1, "mode": "admission_only", "exec_enabled": False}
    row = protocol.handle(envelope(), 1001)
    assert row["state"] == "queued"
    assert row["sha"] == "a" * 40
    assert protocol.handle({"op": "get", "id": row["id"]}, 1001)["state"] == "queued"
    for request in [
        {"op": "run", "command": "id"},
        {"op": "submit", "sha": "a" * 40, "fingerprint": "b" * 64, "hosts": ["semaphore"]},
        {"op": "submit", "ticket": ticket(), "playbook": "../../etc/passwd"},
    ]:
        with pytest.raises(ValueError):
            protocol.handle(request, 1001)


def test_ticket_replay_and_tampering_rejected(store):
    value = ticket()
    first = store.submit(value, 1000, KEY)
    assert first["state"] == "queued"
    store.cancel(first["id"])
    with pytest.raises(ValueError, match="nonce"):
        store.submit(value, 1000, KEY)
    forged = dict(ticket())
    forged["hosts"] = ["other"]
    with pytest.raises(ValueError, match="signature"):
        store.submit(forged, 1000, KEY)
    with pytest.raises(ValueError, match="signature"):
        store.submit(ticket(), 1000, b"Z" * 32)


def test_target_lock_cancel_and_duplicate_rejection(store):
    first = store.submit(ticket(), 1000, KEY)
    with pytest.raises(ValueError, match="locked"):
        store.submit(ticket(), 1000, KEY)
    assert store.cancel(first["id"])["state"] == "cancelled"
    second = store.submit(ticket(), 1000, KEY)
    store.process_pending()
    assert store.get(second["id"])["state"] == "validated"
    assert store.get(first["id"])["state"] == "cancelled"


def test_queue_bound_and_recovery(store):
    for i in range(10):
        store.submit(ticket(["host" + str(i)]), 1000, KEY)
    with pytest.raises(ValueError, match="full"):
        store.submit(ticket(["another"]), 1000, KEY)
    store.recover()
    assert store.get(1)["state"] == "interrupted"
    assert store.submit(ticket(["another"]), 1000, KEY)["state"] == "queued"


@pytest.mark.parametrize("hosts", [
    [], ["h", "h"], ["host;rm"], ["some name"], [".bad"], ["good"] * 21,
])
def test_invalid_host_envelopes_rejected(store, hosts):
    with pytest.raises(ValueError):
        store.submit(ticket(hosts), 1000, KEY)


def test_expired_ticket_and_unknown_profile_rejected(store):
    stale = tickets.issue("a" * 40, "b" * 64, ["semaphore"], "diagnostic",
                          "admin", KEY, now=1)
    with pytest.raises(ValueError, match="expired"):
        store.submit(stale, 1000, KEY)
    value = ticket()
    value["profile"] = "shell"
    value["signature"] = tickets.sign({k: value[k] for k in tickets.FIELDS}, KEY)
    with pytest.raises(ValueError):
        store.submit(value, 1000, KEY)


def test_real_unix_socket_roundtrip(tmp_path, store):
    path = str(tmp_path / "executor.sock")
    with executor_service.Server(
            path, executor_service.Protocol(store, os.geteuid(), KEY)) as server:
        os.chmod(path, 0o660)
        worker = threading.Thread(target=server.serve_forever, daemon=True)
        worker.start()
        try:
            for request, expected in [
                ({"op": "status"}, "admission_only"), (envelope(), "queued")
            ]:
                with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as client:
                    client.settimeout(2)
                    client.connect(path)
                    client.sendall((json.dumps(request) + "\n").encode())
                    received = b""
                    while not received.endswith(b"\n"):
                        received += client.recv(4096)
                assert expected in str(json.loads(received))
            with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as client:
                client.settimeout(2)
                client.connect(path)
                client.sendall(b'{"op":"exec","command":"id"}\n')
                assert b"error" in client.recv(4096)
        finally:
            server.shutdown()
            worker.join(timeout=2)


def test_client_checks_approved_review_and_sha(monkeypatch):
    report = {"repository": "ansible", "inventory": "inventory/hosts.ini",
              "playbook": "playbooks/diag.yml", "group": "lab",
              "mode": "diagnostic", "git_sha": "a" * 40,
              "hosts": ["semaphore"], "eligible": True}
    monkeypatch.setattr(executor_client.preflight, "get", lambda i: {
        "state": "approved", "report": report,
        "fingerprint": "b" * 64, "approved_by": "admin"})
    monkeypatch.setattr(executor_client.preflight, "_assess",
                        lambda *args: (report, "b" * 64))
    monkeypatch.setattr(executor_client.executor_ticket, "load_key", lambda: KEY)
    sent = []
    monkeypatch.setattr(executor_client, "exchange",
                        lambda message: sent.append(message) or {"id": 4, "state": "queued"})
    result = executor_client.submit(7)
    assert result["executable"] is False
    assert len(sent) == 1
    assert set(sent[0]) == {"op", "ticket"}
    assert tickets.verify(sent[0]["ticket"], KEY)["hosts"] == ["semaphore"]
    monkeypatch.setattr(executor_client.preflight, "_assess",
                        lambda *args: (report, "0" * 64))
    with pytest.raises(HTTPException):
        executor_client.submit(7)
    assert len(sent) == 1


def test_client_rejects_unapproved(monkeypatch):
    monkeypatch.setattr(executor_client.preflight, "get",
                        lambda i: {"state": "blocked", "report": {}})
    monkeypatch.setattr(executor_client, "exchange",
                        lambda message: pytest.fail("must not dispatch"))
    with pytest.raises(HTTPException):
        executor_client.submit(7)


def test_missing_socket_fail_closed(monkeypatch, tmp_path):
    monkeypatch.delenv("SAKURAOPS_EXECUTOR_SOCKET", raising=False)
    with pytest.raises(HTTPException):
        executor_client.health()
    monkeypatch.setenv("SAKURAOPS_EXECUTOR_SOCKET", str(tmp_path / "missing.sock"))
    with pytest.raises(HTTPException) as err:
        executor_client.health()
    assert err.value.status_code == 503


def test_separate_user_required_at_service_boundary(tmp_path):
    with pytest.raises(ValueError, match="distinct"):
        executor_service.serve(str(tmp_path / "executor.sock"),
                               str(tmp_path / "executor.sqlite"), os.geteuid())



def test_legacy_receipts_database_migrates(tmp_path):
    import sqlite3
    dbfile = tmp_path / "legacy.sqlite"
    with sqlite3.connect(dbfile) as db:
        db.execute("""CREATE TABLE receipts (
            id INTEGER PRIMARY KEY AUTOINCREMENT, fingerprint TEXT NOT NULL,
            sha TEXT NOT NULL, hosts TEXT NOT NULL, state TEXT NOT NULL,
            created_at TEXT NOT NULL, finished_at TEXT, client_uid INTEGER NOT NULL)""")
        db.execute("""INSERT INTO receipts (
            fingerprint, sha, hosts, state, created_at, client_uid)
            VALUES (?, ?, ?, 'cancelled', 'old-date', 1000)""",
            ("b" * 64, "a" * 40, '["semaphore"]'))
    migrated = executor_service.Store(dbfile)
    assert migrated.get(1)["state"] == "cancelled"
    assert migrated.submit(ticket(), 1000, KEY)["id"] == 2


def test_ticket_private_key_file_permissions(monkeypatch, tmp_path):
    path = tmp_path / "approval.key"
    path.write_bytes(KEY)
    path.chmod(0o600)
    monkeypatch.setenv("SAKURAOPS_EXECUTOR_APPROVAL_KEY", str(path))
    assert tickets.load_key() == KEY
    path.chmod(0o644)
    with pytest.raises(ValueError, match="owner-only"):
        tickets.load_key()
    path.chmod(0o600)
    link = tmp_path / "key-link"
    link.symlink_to(path)
    monkeypatch.setenv("SAKURAOPS_EXECUTOR_APPROVAL_KEY", str(link))
    with pytest.raises(ValueError):
        tickets.load_key()


def test_api_has_no_shell_bridge():
    from pathlib import Path
    root = Path(__file__).resolve().parents[1] / "backend/app"
    api = (root / "main.py").read_text()
    daemon = (root / "executor_service.py").read_text()
    assert '@app.post("/api/executor/receipts"' in api
    assert 'require_console_session(request, edit=True)' in api
    assert "SO_PEERCRED" in daemon
    assert "import subprocess" not in daemon
    assert "subprocess." not in daemon
    assert "ansible-playbook" not in daemon

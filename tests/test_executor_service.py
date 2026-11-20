"""No-network tests for distinct-UID admission-only executor and API bridge."""
import json
import os
import socket
import stat
import struct

import pytest
from fastapi import HTTPException

from backend.app import executor_client, executor_service


@pytest.fixture
def store(tmp_path):
    return executor_service.Store(tmp_path / "receipts.sqlite")


def envelope(hosts=None):
    return {"op": "submit", "fingerprint": "b" * 64, "sha": "a" * 40,
            "hosts": ["semaphore"] if hosts is None else hosts}


def test_only_narrow_protocol_and_authorized_uid(store):
    protocol = executor_service.Protocol(store, 1001)
    with pytest.raises(PermissionError):
        protocol.handle({"op": "status"}, 1002)
    assert protocol.handle({"op": "status"}, 1001) == {
        "version": 1, "mode": "admission_only", "exec_enabled": False}
    row = protocol.handle(envelope(), 1001)
    assert row["state"] == "queued"
    assert row["sha"] == "a" * 40
    assert protocol.handle({"op": "get", "id": row["id"]}, 1001)["state"] == "queued"
    with pytest.raises(ValueError):
        protocol.handle({"op": "run", "command": "id"}, 1001)
    with pytest.raises(ValueError):
        protocol.handle({"op": "submit", "sha": "a" * 40,
                         "fingerprint": "b" * 64, "hosts": ["semaphore"],
                         "playbook": "../../etc/passwd"}, 1001)
    with pytest.raises(ValueError):
        protocol.handle({"op": "submit", "sha": "a" * 40,
                         "fingerprint": "b" * 64, "hosts": []}, 1001)


def test_target_lock_cancel_and_duplicate_rejection(store):
    first = store.submit("b" * 64, "a" * 40, ["semaphore"], 1000)
    with pytest.raises(ValueError, match="locked"):
        store.submit("b" * 64, "a" * 40, ["semaphore"], 1000)
    assert store.cancel(first["id"])["state"] == "cancelled"
    second = store.submit("b" * 64, "a" * 40, ["semaphore"], 1000)
    store.process_pending()
    assert store.get(second["id"])["state"] == "validated"
    assert store.get(first["id"])["state"] == "cancelled"
    assert store.cancel(second["id"])["state"] == "validated"


def test_queue_bound_and_recovery(store):
    for i in range(10):
        store.submit("b" * 64, "a" * 40, ["host" + str(i)], 1000)
    with pytest.raises(ValueError, match="full"):
        store.submit("b" * 64, "a" * 40, ["another"], 1000)
    store.recover()
    assert store.get(1)["state"] == "interrupted"
    assert store.submit("b" * 64, "a" * 40, ["another"], 1000)["state"] == "queued"


@pytest.mark.parametrize("hosts", [
    [], ["h"] * 2, ["host;rm"], ["some name"], [".bad"], ["good"] * 21,
])
def test_invalid_host_envelopes_rejected(store, hosts):
    with pytest.raises(ValueError):
        store.submit("b" * 64, "a" * 40, hosts, 1000)


def test_client_checks_approved_review_and_sha(monkeypatch):
    report = {"repository": "ansible", "inventory": "inventory/hosts.ini",
              "playbook": "playbooks/diag.yml", "group": "lab",
              "mode": "diagnostic", "git_sha": "a" * 40, "hosts": ["semaphore"],
              "eligible": True}
    monkeypatch.setattr(executor_client.preflight, "get", lambda i: {
        "state": "approved", "report": report, "fingerprint": "b" * 64})
    monkeypatch.setattr(executor_client.preflight, "_assess",
                        lambda *args: (report, "b" * 64))
    sent = []
    monkeypatch.setattr(executor_client, "exchange", lambda message: sent.append(message) or
                        {"id": 4, "state": "queued"})
    result = executor_client.submit(7)
    assert result["executable"] is False
    assert sent == [envelope()]
    monkeypatch.setattr(executor_client.preflight, "_assess",
                        lambda *args: (report, "0" * 64))
    with pytest.raises(HTTPException) as err:
        executor_client.submit(7)
    assert err.value.status_code == 409
    assert len(sent) == 1


def test_client_rejects_unapproved(monkeypatch):
    monkeypatch.setattr(executor_client.preflight, "get", lambda i: {
        "state": "blocked", "report": {}})
    monkeypatch.setattr(executor_client, "exchange",
                        lambda message: pytest.fail("must not dispatch"))
    with pytest.raises(HTTPException):
        executor_client.submit(7)


def test_missing_socket_fail_closed(monkeypatch, tmp_path):
    monkeypatch.delenv("SAKURAOPS_EXECUTOR_SOCKET", raising=False)
    with pytest.raises(HTTPException) as err:
        executor_client.health()
    assert err.value.status_code == 409
    monkeypatch.setenv("SAKURAOPS_EXECUTOR_SOCKET", str(tmp_path / "missing.sock"))
    with pytest.raises(HTTPException) as err:
        executor_client.health()
    assert err.value.status_code == 503


def test_separate_user_required_at_service_boundary(tmp_path):
    with pytest.raises(ValueError, match="distinct"):
        executor_service.serve(str(tmp_path / "executor.sock"),
                               str(tmp_path / "executor.sqlite"), os.geteuid())


def test_api_has_no_shell_bridge():
    from pathlib import Path
    api = (Path(__file__).resolve().parents[1] / "backend/app/main.py").read_text()
    client = (Path(__file__).resolve().parents[1] / "backend/app/executor_client.py").read_text()
    daemon = (Path(__file__).resolve().parents[1] / "backend/app/executor_service.py").read_text()
    assert '@app.post("/api/executor/receipts"' in api
    assert 'require_console_session(request, edit=True)' in api
    assert "SO_PEERCRED" in client
    assert "SO_PEERCRED" in daemon
    assert "import subprocess" not in daemon
    assert "subprocess." not in daemon
    assert "ansible-playbook" not in daemon

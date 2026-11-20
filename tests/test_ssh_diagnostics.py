"""SSH diagnostic safety tests. No network, credentials or real SSH operations."""
import io
import os
import subprocess
import pytest
from fastapi import HTTPException
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from backend.app import ssh_diagnostics
from backend.app.models import SSHDiagnosticJob


@pytest.fixture
def diag_db(tmp_path, monkeypatch):
    engine = create_engine("sqlite://", connect_args={"check_same_thread": False},
                           poolclass=StaticPool)
    SSHDiagnosticJob.__table__.create(bind=engine)
    monkeypatch.setattr(ssh_diagnostics, "SessionLocal", sessionmaker(bind=engine))
    monkeypatch.setattr(ssh_diagnostics, "LOCK_PATH", tmp_path / "ssh.lock")
    monkeypatch.setenv("SAKURAOPS_SSH_DIAG_ENABLED", "true")
    monkeypatch.setenv("SAKURAOPS_SSH_DIAG_TARGETS",
                       "glpi=192.0.2.10,libre=198.51.100.7")
    key = tmp_path / "identity"
    known = tmp_path / "known_hosts"
    key.write_text("fake-key-test")
    key.chmod(0o600)
    known.write_text("192.0.2.10 ssh-ed25519 fake")
    known.chmod(0o600)
    monkeypatch.setenv("SAKURAOPS_SSH_DIAG_KEY", str(key))
    monkeypatch.setenv("SAKURAOPS_SSH_DIAG_KNOWN_HOSTS", str(known))
    ssh_diagnostics.STOP.clear()
    yield key, known
    ssh_diagnostics.STOP.clear()
    engine.dispose()


def test_disabled_without_opt_in_and_disallows_any_target(diag_db, monkeypatch):
    monkeypatch.delenv("SAKURAOPS_SSH_DIAG_ENABLED")
    assert ssh_diagnostics.available() is False
    with pytest.raises(HTTPException) as exc:
        ssh_diagnostics.enqueue("glpi")
    assert exc.value.status_code == 409


@pytest.mark.parametrize("targets", [
    "glpi=internal-host.example.org",
    "glpi=127.0.0.1",
    "glpi=0.0.0.0",
    "glpi=224.0.0.1",
    "glpi=192.0.2.10,glpi=192.0.2.11",
    "glpi=192.0.2.10,broken",
    "glpi=192.0.2.10;touch /tmp/x",
])
def test_invalid_targets_fail_closed(diag_db, monkeypatch, targets):
    monkeypatch.setenv("SAKURAOPS_SSH_DIAG_TARGETS", targets)
    with pytest.raises(HTTPException):
        ssh_diagnostics.configuration()


def test_credentials_reject_symlink_or_insecure_key(diag_db, tmp_path, monkeypatch):
    key, known = diag_db
    key.chmod(0o644)
    assert not ssh_diagnostics.available()
    key.chmod(0o600)
    link = tmp_path / "symlink"
    link.symlink_to(key)
    monkeypatch.setenv("SAKURAOPS_SSH_DIAG_KEY", str(link))
    assert not ssh_diagnostics.available()
    monkeypatch.setenv("SAKURAOPS_SSH_DIAG_KEY", str(key))
    known.chmod(0o666)
    assert not ssh_diagnostics.available()


def test_queued_workflow_and_allowlist_gate(diag_db):
    config = ssh_diagnostics.configuration()
    assert config["enabled"] is True
    assert config["account"] == "sakuraops-diag"
    assert len(config["targets"]) == 2
    with pytest.raises(HTTPException) as error:
        ssh_diagnostics.enqueue("not-approved")
    assert error.value.status_code == 403
    item = ssh_diagnostics.enqueue("glpi")
    assert item["address"] == "192.0.2.10"
    assert "key" not in str(item)
    assert ssh_diagnostics.list_jobs()[0]["id"] == item["id"]
    assert ssh_diagnostics.cancel_job(item["id"])["status"] == "cancelled"
    assert ssh_diagnostics.get_job(item["id"])["status"] == "cancelled"


def test_run_uses_pinned_keys_noninteractive_ssh_only(diag_db, monkeypatch):
    item = ssh_diagnostics.enqueue("glpi")
    with ssh_diagnostics.SessionLocal() as db:
        row = db.get(SSHDiagnosticJob, item["id"])
        row.status = "running"
        row.started_at = ssh_diagnostics._now()
        db.commit()
    observed = []

    class FakeProcess:
        returncode = 0
        stdout = io.BytesIO(b"SSH connected\n")
        def poll(self):
            return 0
        def wait(self, timeout=None):
            return 0

    def fake_popen(command, **kwargs):
        observed.append((command, kwargs))
        return FakeProcess()

    monkeypatch.setattr(ssh_diagnostics.subprocess, "Popen", fake_popen)
    ssh_diagnostics._run(item["id"])
    result = ssh_diagnostics.get_job(item["id"])
    assert result["status"] == "success"
    assert "SSH connected" in result["log"]
    cmd, kwargs = observed[0]
    assert cmd[0] == "/usr/bin/ssh"
    assert cmd[-2:] == ["192.0.2.10", "true"]
    assert "-F" in cmd and cmd[cmd.index("-F") + 1] == "/dev/null"
    assert "StrictHostKeyChecking=yes" in cmd
    assert "BatchMode=yes" in cmd
    assert "UserKnownHostsFile=" in " ".join(cmd)
    assert cmd[cmd.index("-l") + 1] == "sakuraops-diag"
    assert "-i" in cmd
    assert kwargs["stdin"] == subprocess.DEVNULL
    assert kwargs["env"]["HOME"] == "/nonexistent"
    assert "ANSIBLE_CONFIG" not in kwargs["env"]


def test_revoked_allowlist_refuses_queued_job(diag_db, monkeypatch):
    item = ssh_diagnostics.enqueue("glpi")
    with ssh_diagnostics.SessionLocal() as db:
        row = db.get(SSHDiagnosticJob, item["id"])
        row.status = "running"
        db.commit()
    monkeypatch.setenv("SAKURAOPS_SSH_DIAG_TARGETS", "libre=198.51.100.7")
    monkeypatch.setattr(ssh_diagnostics.subprocess, "Popen",
                        lambda *args, **kwargs: pytest.fail("SSH must not be started"))
    ssh_diagnostics._run(item["id"])
    assert ssh_diagnostics.get_job(item["id"])["status"] == "failed"
    assert "Odmowa" in ssh_diagnostics.get_job(item["id"])["log"]


def test_api_and_ui_wired_for_admin_only():
    from pathlib import Path
    root = Path(__file__).resolve().parents[1]
    main = (root / "backend/app/main.py").read_text()
    html = (root / "backend/app/static/dashboard.html").read_text()
    js = (root / "frontend/src/terminal.js").read_text()
    assert '@app.post("/api/ssh-diagnostics/jobs"' in main
    assert 'require_console_session(request, edit=True)' in main
    assert 'ssh_diagnostics.stop_worker()' in main
    assert 'id="ssh-diagnostic-target"' in html
    assert 'id="ssh-diagnostic-start"' in html
    assert 'new EventSource("/api/ssh-diagnostics/jobs/"' in js

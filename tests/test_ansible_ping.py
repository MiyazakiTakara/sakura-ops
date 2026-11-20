"""No-network regression tests for Ansible ping diagnostics."""
from pathlib import Path
import subprocess
import sys

import pytest
from fastapi import HTTPException
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from backend.app import ansible_ping
from backend.app.models import AnsiblePingJob


@pytest.fixture
def ping_env(monkeypatch, tmp_path):
    db = create_engine("sqlite://", connect_args={"check_same_thread": False},
                       poolclass=StaticPool)
    AnsiblePingJob.__table__.create(db)
    monkeypatch.setattr(ansible_ping, "SessionLocal", sessionmaker(bind=db))
    monkeypatch.setattr(ansible_ping, "LOCK_PATH", tmp_path / "worker.lock")
    monkeypatch.setenv("SAKURAOPS_ANSIBLE_PING_ENABLED", "true")
    monkeypatch.setenv("SAKURAOPS_SSH_DIAG_TARGETS", "semaphore=192.0.2.181")
    key = tmp_path / "ping_ed25519"
    known = tmp_path / "known_hosts"
    key.write_text("fake-private-key")
    key.chmod(0o600)
    known.write_text("192.0.2.181 ssh-ed25519 FAKE\n")
    known.chmod(0o600)
    monkeypatch.setenv("SAKURAOPS_ANSIBLE_PING_KEY", str(key))
    monkeypatch.setenv("SAKURAOPS_ANSIBLE_PING_KNOWN_HOSTS", str(known))
    ansible_ping.STOP.clear()
    yield key, known
    ansible_ping.STOP.clear()
    db.dispose()


def test_disabled_by_default(monkeypatch):
    monkeypatch.delenv("SAKURAOPS_ANSIBLE_PING_ENABLED", raising=False)
    assert ansible_ping.enabled() is False
    with pytest.raises(HTTPException) as ex:
        ansible_ping.enqueue("semaphore")
    assert ex.value.status_code == 409


def test_allowlist_and_cancel(ping_env):
    assert ansible_ping.configuration()["enabled"] is True
    with pytest.raises(HTTPException) as ex:
        ansible_ping.enqueue("not-approved")
    assert ex.value.status_code == 403
    job = ansible_ping.enqueue("semaphore")
    assert job["status"] == "queued"
    assert job["address"] == "192.0.2.181"
    assert ansible_ping.cancel(job["id"])["status"] == "cancelled"
    assert ansible_ping.get_job(job["id"])["status"] == "cancelled"


def test_insecure_key_and_symlink_rejected(ping_env, monkeypatch, tmp_path):
    key, known = ping_env
    key.chmod(0o644)
    assert not ansible_ping.enabled()
    key.chmod(0o600)
    link = tmp_path / "symlink"
    link.symlink_to(key)
    monkeypatch.setenv("SAKURAOPS_ANSIBLE_PING_KEY", str(link))
    assert not ansible_ping.enabled()
    monkeypatch.setenv("SAKURAOPS_ANSIBLE_PING_KEY", str(key))
    known.write_text("")
    assert not ansible_ping.enabled()


def test_literal_builtin_module_and_own_identity(ping_env, monkeypatch):
    job = ansible_ping.enqueue("semaphore")
    with ansible_ping.SessionLocal() as db:
        row = db.get(AnsiblePingJob, job["id"])
        row.status = "running"
        db.commit()

    real_popen = subprocess.Popen
    calls = []

    def fake_popen(command, **kw):
        calls.append((command, kw))
        config = Path(kw["env"]["ANSIBLE_CONFIG"])
        assert config.is_file()
        assert config.stat().st_mode & 0o777 == 0o600
        cfg = config.read_text()
        assert "StrictHostKeyChecking=yes" in cfg
        assert "ControlMaster=no" in cfg
        assert "usetty = False" in cfg
        assert "ssh_args = -T " in cfg
        assert kw["env"]["ANSIBLE_SSH_USETTY"] == "false"
        assert str(ping_env[1]) in cfg
        return real_popen([sys.executable, "-c", "print('SUCCESS => pong')"], **kw)

    monkeypatch.setattr(ansible_ping.shutil, "which", lambda name: "/venv/bin/ansible")
    monkeypatch.setattr(ansible_ping.subprocess, "Popen", fake_popen)
    ansible_ping._run(job["id"])
    result = ansible_ping.get_job(job["id"])
    assert result["status"] == "success"
    assert result["exit_code"] == 0
    assert "pong" in result["log"]
    cmd, kw = calls[0]
    assert cmd == ["/venv/bin/ansible", "-i", "192.0.2.181,", "all",
                   "-m", "ansible.builtin.ping", "-u", "sakuraops-ping",
                   "--private-key", str(ping_env[0]),
                   "--forks", "1", "--timeout", "8"]
    assert kw["stdin"] == subprocess.DEVNULL
    assert kw["env"]["HOME"] != str(Path.home())


def test_revoked_target_never_invokes_ansible(ping_env, monkeypatch):
    job = ansible_ping.enqueue("semaphore")
    with ansible_ping.SessionLocal() as db:
        row = db.get(AnsiblePingJob, job["id"])
        row.status = "running"
        db.commit()
    monkeypatch.setenv("SAKURAOPS_SSH_DIAG_TARGETS", "other=198.51.100.25")
    monkeypatch.setattr(ansible_ping.subprocess, "Popen",
                        lambda *args, **kw: pytest.fail("must not execute"))
    ansible_ping._run(job["id"])
    assert ansible_ping.get_job(job["id"])["status"] == "failed"


def test_api_and_ui_paths():
    root = Path(__file__).resolve().parents[1]
    main = (root / "backend/app/main.py").read_text()
    html = (root / "backend/app/static/dashboard.html").read_text()
    js = (root / "frontend/src/terminal.js").read_text()
    assert '@app.post("/api/ansible-ping/jobs"' in main
    assert 'require_console_session(request, edit=True)' in main
    assert "ansible_ping.start_worker()" in main
    assert "ansible_ping.stop_worker()" in main
    assert 'id="ansible-ping-start"' in html
    assert 'new EventSource("/api/ansible-ping/jobs/"' in js

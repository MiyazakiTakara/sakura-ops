"""Local Ansible job executor tests — no remote hosts or user playbooks."""
import subprocess
import sys

import pytest
from fastapi import HTTPException
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from backend.app import local_jobs
from backend.app.models import LocalTestJob
from backend.app.commands import interpret


@pytest.fixture
def jobs_db(monkeypatch, tmp_path):
    engine = create_engine("sqlite://", connect_args={"check_same_thread": False},
                           poolclass=StaticPool)
    LocalTestJob.__table__.create(bind=engine)
    monkeypatch.setattr(local_jobs, "SessionLocal", sessionmaker(bind=engine))
    monkeypatch.setattr(local_jobs, "LOCK_PATH", tmp_path / "local-worker.lock")
    monkeypatch.setenv("SAKURAOPS_LOCAL_TESTS_ENABLED", "true")
    local_jobs.STOP.clear()
    yield
    local_jobs.STOP.clear()
    engine.dispose()


def test_local_job_disabled_by_default(monkeypatch):
    monkeypatch.delenv("SAKURAOPS_LOCAL_TESTS_ENABLED", raising=False)
    assert local_jobs.enabled() is False
    with pytest.raises(HTTPException) as error:
        local_jobs.enqueue()
    assert error.value.status_code == 409


def test_enqueue_queue_list_cancel_and_history(jobs_db):
    first = local_jobs.enqueue()
    assert first["status"] == "queued"
    assert first["type"] == "local_ansible_test"
    assert "repository" not in first
    assert local_jobs.list_jobs()[0]["id"] == first["id"]
    cancelled = local_jobs.cancel_job(first["id"])
    assert cancelled["status"] == "cancelled"
    assert "anulowany" in local_jobs.get_job(first["id"])["log"]
    assert local_jobs.cancel_job(first["id"])["status"] == "cancelled"
    with pytest.raises(HTTPException):
        local_jobs.get_job(9999)


def test_queue_limit(jobs_db):
    for _ in range(10):
        local_jobs.enqueue()
    with pytest.raises(HTTPException) as exc:
        local_jobs.enqueue()
    assert exc.value.status_code == 429


def test_local_runner_streams_log_and_finishes(jobs_db, monkeypatch):
    row = local_jobs.enqueue()
    with local_jobs.SessionLocal() as db:
        job = db.get(LocalTestJob, row["id"])
        job.status = "running"
        job.started_at = local_jobs._now()
        db.commit()
    original_popen = subprocess.Popen
    observed = []

    def fake_popen(command, **kwargs):
        observed.append((command, kwargs))
        # Execute a fixed local print only; never run or require Ansible in CI.
        return original_popen([sys.executable, "-c", "print('LOCAL SMOKE PASS')"], **kwargs)

    monkeypatch.setattr(local_jobs.subprocess, "Popen", fake_popen)
    local_jobs._run(row["id"])
    finished = local_jobs.get_job(row["id"])
    assert finished["status"] == "success"
    assert finished["exit_code"] == 0
    assert "LOCAL SMOKE PASS" in finished["log"]
    command, kwargs = observed[0]
    assert command[0] == "ansible-playbook"
    assert "--check" in command
    assert command[command.index("--limit") + 1] == "localhost"
    assert "localhost," in command
    assert "inventory/hosts.ini" not in command
    assert "playbooks/linux-inventory.yml" not in command
    assert kwargs["stdin"] == subprocess.DEVNULL
    assert kwargs["env"]["ANSIBLE_CONFIG"] == "/dev/null"


def test_worker_handles_queued_cancel_without_starting_job(jobs_db):
    entry = local_jobs.enqueue()
    local_jobs.cancel_job(entry["id"])
    assert local_jobs.get_job(entry["id"])["status"] == "cancelled"


def test_console_and_api_wiring():
    assert interpret("task test")["action"] == "local-test-create"
    assert interpret("show jobs")["action"] == "local-test-list"
    assert interpret("show job 7")["job_id"] == 7
    assert interpret("task cancel 7")["action"] == "local-test-cancel"
    assert "Nieznane polecenie" in interpret("task cancel localhost")["output"]
    from pathlib import Path
    root = Path(__file__).resolve().parents[1]
    api = (root / "backend/app/main.py").read_text()
    page = (root / "backend/app/static/dashboard.html").read_text()
    js = (root / "frontend/src/terminal.js").read_text()
    assert '@app.post("/api/jobs/local-test"' in api
    assert 'require_console_session(request, edit=True)' in api
    assert 'media_type="text/event-stream"' in api
    for label in ("local-test-start", "local-job-log", "local-job-cancel"):
        assert f'id="{label}"' in page
    assert 'new EventSource("/api/jobs/"' in js

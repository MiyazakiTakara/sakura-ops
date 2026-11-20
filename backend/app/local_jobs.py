"""Localhost-only Ansible smoke executor. No user playbook or inventory execution.

A single daemon worker, guarded by an OS lock, consumes SQLite jobs. The only
subprocess is a fixed packaged playbook with localhost and connection=local.
"""
import fcntl
import os
import resource
import selectors
import shutil
import signal
import subprocess
import tempfile
import threading
import time
from datetime import datetime, timezone
from pathlib import Path

from fastapi import HTTPException
from sqlalchemy import select

from .database import DATA_DIR, SessionLocal, engine
from .models import LocalTestJob

LocalTestJob.__table__.create(bind=engine, checkfirst=True)

LOCK_PATH = DATA_DIR / "local-test-worker.lock"
PLAYBOOK = Path(__file__).with_name("local_test.yml")
TERMINAL = {"success", "failed", "cancelled"}
MAX_LOG = 65536
JOB_TIMEOUT = 60
STOP = threading.Event()
WORKER = None


def enabled() -> bool:
    return os.getenv("SAKURAOPS_LOCAL_TESTS_ENABLED", "").lower() == "true"


def _now() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


def _as_dict(row: LocalTestJob, include_log: bool = False) -> dict:
    value = {"id": row.id, "type": "local_ansible_test", "status": row.status,
             "created_at": row.created_at, "started_at": row.started_at,
             "finished_at": row.finished_at, "exit_code": row.exit_code,
             "cancel_requested": row.cancel_requested}
    if include_log:
        value["log"] = row.log
    return value


def enqueue() -> dict:
    if not enabled():
        raise HTTPException(409, "Testy lokalne są wyłączone (SAKURAOPS_LOCAL_TESTS_ENABLED)")
    with SessionLocal() as db:
        waiting = db.scalars(select(LocalTestJob).where(
            LocalTestJob.status.in_(("queued", "running")))).all()
        if len(waiting) >= 10:
            raise HTTPException(429, "Kolejka testów jest pełna")
        job = LocalTestJob(status="queued", created_at=_now(), log="")
        db.add(job)
        db.commit()
        db.refresh(job)
        return _as_dict(job)


def list_jobs() -> list[dict]:
    with SessionLocal() as db:
        rows = db.scalars(select(LocalTestJob).order_by(LocalTestJob.id.desc()).limit(50))
        return [_as_dict(row) for row in rows]


def get_job(job_id: int) -> dict:
    with SessionLocal() as db:
        row = db.get(LocalTestJob, job_id)
        if row is None:
            raise HTTPException(404, "Nie znaleziono zadania")
        return _as_dict(row, include_log=True)


def cancel_job(job_id: int) -> dict:
    with SessionLocal() as db:
        row = db.get(LocalTestJob, job_id)
        if row is None:
            raise HTTPException(404, "Nie znaleziono zadania")
        if row.status in TERMINAL:
            return _as_dict(row)
        row.cancel_requested = True
        if row.status == "queued":
            row.status = "cancelled"
            row.finished_at = _now()
            row.log = "[SakuraOps] Test anulowany przed uruchomieniem.\n"
        db.commit()
        db.refresh(row)
        return _as_dict(row)


def _append(job_id: int, data: str) -> None:
    with SessionLocal() as db:
        row = db.get(LocalTestJob, job_id)
        if row and row.status == "running":
            combined = row.log + data
            row.log = combined[-MAX_LOG:]
            db.commit()


def _limits() -> None:
    resource.setrlimit(resource.RLIMIT_CPU, (30, 35))
    resource.setrlimit(resource.RLIMIT_AS, (1024 * 1024 * 1024, 1024 * 1024 * 1024))
    resource.setrlimit(resource.RLIMIT_FSIZE, (1024 * 1024, 1024 * 1024))


def _run(job_id: int) -> None:
    proc = None
    outcome = "failed"
    code = None
    with tempfile.TemporaryDirectory(prefix="sakura-ansible-") as directory:
        # Ansible requires a recognized configuration extension: /dev/null
        # is rejected by modern ansible-core before any playbook can run.
        config_file = Path(directory) / "ansible.cfg"
        config_file.write_text(
            "[defaults]\nretry_files_enabled = False\n",
            encoding="utf-8",
        )
        config_file.chmod(0o600)
        executable = shutil.which("ansible-playbook")
        command = [
            executable or "ansible-playbook", "--check", "-i", "localhost,", "-c", "local",
            "--limit", "localhost", str(PLAYBOOK),
        ]
        env = {
            "PATH": "/usr/local/bin:/usr/bin:/bin",
            "HOME": directory,
            "LANG": "C.UTF-8",
            "ANSIBLE_CONFIG": str(config_file),
            "ANSIBLE_LOCAL_TEMP": directory,
            "ANSIBLE_NOCOLOR": "1",
            "PYTHONNOUSERSITE": "1",
        }
        try:
            if executable is None:
                raise FileNotFoundError("ansible-playbook")
            proc = subprocess.Popen(
                command, cwd=directory, env=env, stdin=subprocess.DEVNULL,
                stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                start_new_session=True, preexec_fn=_limits,
            )
            _append(job_id, "[SakuraOps] Lokalny test localhost (check mode).\n")
            deadline = time.monotonic() + JOB_TIMEOUT
            with selectors.DefaultSelector() as selector:
                os.set_blocking(proc.stdout.fileno(), False)
                selector.register(proc.stdout, selectors.EVENT_READ)
                while True:
                    with SessionLocal() as db:
                        current = db.get(LocalTestJob, job_id)
                        cancelled = bool(current and current.cancel_requested)
                    if cancelled or STOP.is_set() or time.monotonic() > deadline:
                        os.killpg(proc.pid, signal.SIGKILL)
                        outcome = "cancelled" if cancelled else "failed"
                        _append(job_id, "[SakuraOps] Przerwano lub przekroczono limit czasu.\n")
                        break
                    for key, _ in selector.select(timeout=0.2):
                        chunk = os.read(key.fileobj.fileno(), 4096)
                        if chunk:
                            _append(job_id, chunk.decode("utf-8", errors="replace"))
                        else:
                            selector.unregister(key.fileobj)
                    if proc.poll() is not None:
                        # Drain output written immediately before process exit.
                        for chunk in iter(lambda: os.read(proc.stdout.fileno(), 4096), b""):
                            _append(job_id, chunk.decode("utf-8", errors="replace"))
                        code = proc.returncode
                        outcome = "success" if code == 0 else "failed"
                        break
            proc.wait(timeout=5)
        except FileNotFoundError:
            _append(job_id, "[SakuraOps] Brak ansible-playbook. Zainstaluj Ansible dla testów lokalnych.\n")
        except (OSError, subprocess.TimeoutExpired) as exc:
            _append(job_id, "[SakuraOps] Lokalny proces testowy nie mógł zakończyć się poprawnie.\n")
            if proc is not None and proc.poll() is None:
                os.killpg(proc.pid, signal.SIGKILL)
                proc.wait()
        finally:
            if proc is not None and proc.stdout is not None:
                proc.stdout.close()
    with SessionLocal() as db:
        row = db.get(LocalTestJob, job_id)
        if row:
            row.status = "cancelled" if row.cancel_requested else outcome
            row.exit_code = code
            row.finished_at = _now()
            db.commit()


def _worker() -> None:
    fd = os.open(LOCK_PATH, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return
        with SessionLocal() as db:
            for job in db.scalars(select(LocalTestJob).where(LocalTestJob.status == "running")):
                job.status = "failed"
                job.finished_at = _now()
                job.log = (job.log + "\n[SakuraOps] Proces przerwany przez restart serwera.\n")[-MAX_LOG:]
            db.commit()
        while not STOP.is_set():
            job_id = None
            with SessionLocal() as db:
                job = db.scalar(select(LocalTestJob).where(
                    LocalTestJob.status == "queued").order_by(LocalTestJob.id))
                if job:
                    job.status = "running"
                    job.started_at = _now()
                    job_id = job.id
                    db.commit()
            if job_id is None:
                STOP.wait(0.3)
                continue
            try:
                _run(job_id)
            except Exception:
                with SessionLocal() as db:
                    job = db.get(LocalTestJob, job_id)
                    if job:
                        job.status = "failed"
                        job.finished_at = _now()
                        job.log = (job.log + "\n[SakuraOps] Błąd wykonawcy.\n")[-MAX_LOG:]
                        db.commit()
    finally:
        os.close(fd)


def start_worker() -> None:
    global WORKER
    if not enabled():
        return
    if WORKER and WORKER.is_alive():
        return
    STOP.clear()
    WORKER = threading.Thread(target=_worker, daemon=True, name="sakura-local-test")
    WORKER.start()


def stop_worker() -> None:
    STOP.set()
    if WORKER and WORKER.is_alive():
        WORKER.join(timeout=3)

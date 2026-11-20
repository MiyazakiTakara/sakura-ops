"""Explicit opt-in Ansible builtin ping with a separate low-privilege SSH identity.

Never executes repository playbooks, loads inventory files, or accepts module
arguments. Ansible ping does run transient Python code on the remote host.
"""
import fcntl
import os
from pathlib import Path
import resource
import selectors
import shutil
import signal
import subprocess
import tempfile
import threading
import time

from fastapi import HTTPException
from sqlalchemy import select

from .database import DATA_DIR, SessionLocal, engine
from .models import AnsiblePingJob
from . import ssh_diagnostics as ssh

AnsiblePingJob.__table__.create(bind=engine, checkfirst=True)

LOCK_PATH = DATA_DIR / "ansible-ping-worker.lock"
USER = "sakuraops-ping"
MAX_LOG = 16384
TIMEOUT = 45
STOP = threading.Event()
WORKER = None
END = ("success", "failed", "cancelled")


def _targets():
    return ssh._targets_from_config()


def _credentials():
    key = ssh._owned_regular_file(os.getenv("SAKURAOPS_ANSIBLE_PING_KEY", ""), private=True)
    known = ssh._owned_regular_file(os.getenv("SAKURAOPS_ANSIBLE_PING_KNOWN_HOSTS", ""), private=False)
    if not key or not known:
        return None
    # Ansible's ssh_args is parsed as a shell-like command string. Reject
    # whitespace/metacharacters instead of interpolating attacker-shaped paths.
    for path in (key, known):
        if any(c not in "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._-" for c in str(path)):
            return None
    if not known.stat().st_size:
        return None
    return key, known


def enabled():
    return (os.getenv("SAKURAOPS_ANSIBLE_PING_ENABLED", "").lower() == "true"
            and bool(_targets()) and _credentials() is not None)


def configuration():
    targets = _targets()
    return {"enabled": enabled(), "account": USER,
            "targets": [{"alias": k, "address": v} for k, v in sorted(targets.items())],
            "credentials_ready": _credentials() is not None}


def _dto(row, include_log=False):
    result = {"id": row.id, "type": "ansible_builtin_ping", "alias": row.alias,
              "address": row.address, "status": row.status,
              "created_at": row.created_at, "started_at": row.started_at,
              "finished_at": row.finished_at, "exit_code": row.exit_code,
              "cancel_requested": row.cancel_requested}
    if include_log:
        result["log"] = row.log
    return result


def enqueue(alias):
    if not enabled():
        raise HTTPException(409, "Diagnostyka Ansible ping wyłączona lub nieprzygotowana")
    targets = _targets()
    if alias not in targets:
        raise HTTPException(403, "Host poza listą dozwolonych")
    with SessionLocal() as db:
        count = len(db.scalars(select(AnsiblePingJob).where(
            AnsiblePingJob.status.in_(("queued", "running")))).all())
        if count >= 10:
            raise HTTPException(429, "Kolejka Ansible ping jest pełna")
        job = AnsiblePingJob(alias=alias, address=targets[alias],
                            created_at=ssh._now(), status="queued", log="")
        db.add(job)
        db.commit()
        db.refresh(job)
        return _dto(job)


def list_jobs():
    with SessionLocal() as db:
        return [_dto(j) for j in db.scalars(
            select(AnsiblePingJob).order_by(AnsiblePingJob.id.desc()).limit(50))]


def get_job(job_id):
    with SessionLocal() as db:
        job = db.get(AnsiblePingJob, job_id)
        if job is None:
            raise HTTPException(404, "Nie znaleziono testu Ansible ping")
        return _dto(job, True)


def cancel(job_id):
    with SessionLocal() as db:
        job = db.get(AnsiblePingJob, job_id)
        if job is None:
            raise HTTPException(404, "Nie znaleziono testu Ansible ping")
        if job.status in END:
            return _dto(job)
        job.cancel_requested = True
        if job.status == "queued":
            job.status = "cancelled"
            job.finished_at = ssh._now()
            job.log = "[SakuraOps] Anulowano przed uruchomieniem.\n"
        db.commit()
        db.refresh(job)
        return _dto(job)


def _append(job_id, text):
    with SessionLocal() as db:
        job = db.get(AnsiblePingJob, job_id)
        if job and job.status == "running":
            job.log = (job.log + text)[-MAX_LOG:]
            db.commit()


def _limits():
    resource.setrlimit(resource.RLIMIT_CPU, (25, 30))
    resource.setrlimit(resource.RLIMIT_FSIZE, (1024 * 1024, 1024 * 1024))


def _command(executable, address, key):
    return [executable, "-i", address + ",", "all",
            "-m", "ansible.builtin.ping", "-u", USER, "--private-key", str(key),
            "--one-line", "--forks", "1", "--timeout", "8"]


def _run(job_id):
    with SessionLocal() as db:
        job = db.get(AnsiblePingJob, job_id)
        if job is None:
            return
        alias, address = job.alias, job.address

    outcome, code = "failed", None
    if not enabled() or _targets().get(alias) != address:
        _append(job_id, "[SakuraOps] Odmowa: host lub konfiguracja uległy zmianie.\n")
    else:
        credentials = _credentials()
        executable = shutil.which("ansible")
        if not credentials or not executable:
            _append(job_id, "[SakuraOps] Brak bezpiecznych poświadczeń lub ansible CLI.\n")
        else:
            key, known = credentials
            with tempfile.TemporaryDirectory(prefix="sakura-ping-") as directory:
                # No system, repository, or user inventory or Ansible config.
                config = Path(directory) / "ansible.cfg"
                config.write_text(
                    "[defaults]\n"
                    "retry_files_enabled = False\n"
                    "host_key_checking = True\n"
                    "interpreter_python = auto_silent\n"
                    "[ssh_connection]\n"
                    "ssh_args = -o BatchMode=yes -o StrictHostKeyChecking=yes "
                    "-o UserKnownHostsFile=" + str(known)
                    + " -o IdentitiesOnly=yes -o IdentityAgent=none"
                    + " -o ForwardAgent=no -o ForwardX11=no"
                    + " -o ClearAllForwardings=yes -o ControlMaster=no\n",
                    encoding="utf-8")
                config.chmod(0o600)
                env = {"PATH": os.path.dirname(executable) + ":/usr/bin:/bin",
                       "HOME": directory, "LANG": "C.UTF-8",
                       "ANSIBLE_CONFIG": str(config),
                       "ANSIBLE_LOCAL_TEMP": directory,
                       "ANSIBLE_NOCOLOR": "1", "PYTHONNOUSERSITE": "1",
                       "ANSIBLE_HOST_KEY_CHECKING": "True"}
                command = _command(executable, address, key)
                _append(job_id, "[SakuraOps] Ansible builtin ping: " + alias
                        + " (" + address + "), user " + USER + ".\n")
                proc = None
                try:
                    proc = subprocess.Popen(
                        command, env=env, cwd=directory, stdin=subprocess.DEVNULL,
                        stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                        start_new_session=True, preexec_fn=_limits)
                    deadline = time.monotonic() + TIMEOUT
                    with selectors.DefaultSelector() as sel:
                        os.set_blocking(proc.stdout.fileno(), False)
                        sel.register(proc.stdout, selectors.EVENT_READ)
                        while True:
                            with SessionLocal() as db:
                                current = db.get(AnsiblePingJob, job_id)
                                cancelled = current is None or current.cancel_requested
                            if cancelled or STOP.is_set() or time.monotonic() >= deadline:
                                os.killpg(proc.pid, signal.SIGKILL)
                                outcome = "cancelled" if cancelled else "failed"
                                _append(job_id, "[SakuraOps] Przerwano lub przekroczono limit.\n")
                                break
                            for item, _ in sel.select(timeout=0.2):
                                chunk = os.read(item.fileobj.fileno(), 4096)
                                if chunk:
                                    _append(job_id, chunk.decode("utf-8", "replace"))
                                else:
                                    sel.unregister(item.fileobj)
                            if proc.poll() is not None:
                                while True:
                                    try:
                                        data = os.read(proc.stdout.fileno(), 4096)
                                    except BlockingIOError:
                                        break
                                    if not data:
                                        break
                                    _append(job_id, data.decode("utf-8", "replace"))
                                code = proc.returncode
                                outcome = "success" if code == 0 else "failed"
                                break
                    proc.wait(timeout=3)
                except (OSError, subprocess.TimeoutExpired):
                    _append(job_id, "[SakuraOps] Błąd lokalnego procesu Ansible.\n")
                finally:
                    if proc and proc.poll() is None:
                        os.killpg(proc.pid, signal.SIGKILL)
                        proc.wait(timeout=3)
                    if proc and proc.stdout:
                        proc.stdout.close()
    with SessionLocal() as db:
        job = db.get(AnsiblePingJob, job_id)
        if job:
            job.status = "cancelled" if job.cancel_requested else outcome
            job.exit_code = code
            job.finished_at = ssh._now()
            db.commit()


def _worker():
    fd = os.open(LOCK_PATH, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return
        with SessionLocal() as db:
            for job in db.scalars(select(AnsiblePingJob).where(AnsiblePingJob.status == "running")):
                job.status = "failed"
                job.finished_at = ssh._now()
                job.log = (job.log + "\n[SakuraOps] Restart wykonawcy.\n")[-MAX_LOG:]
            db.commit()
        while not STOP.is_set():
            with SessionLocal() as db:
                job = db.scalar(select(AnsiblePingJob).where(
                    AnsiblePingJob.status == "queued").order_by(AnsiblePingJob.id))
                if job is None:
                    job_id = None
                else:
                    job.status = "running"
                    job.started_at = ssh._now()
                    job_id = job.id
                    db.commit()
            if job_id is None:
                STOP.wait(0.3)
            else:
                try:
                    _run(job_id)
                except Exception:
                    with SessionLocal() as db:
                        job = db.get(AnsiblePingJob, job_id)
                        if job:
                            job.status = "failed"
                            job.finished_at = ssh._now()
                            job.log = (job.log + "\n[SakuraOps] Błąd wykonawcy.\n")[-MAX_LOG:]
                            db.commit()
    finally:
        os.close(fd)


def start_worker():
    global WORKER
    if not enabled() or (WORKER and WORKER.is_alive()):
        return
    STOP.clear()
    WORKER = threading.Thread(target=_worker, daemon=True, name="sakura-ansible-ping")
    WORKER.start()


def stop_worker():
    STOP.set()
    if WORKER and WORKER.is_alive():
        WORKER.join(timeout=3)

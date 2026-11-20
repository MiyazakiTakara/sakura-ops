"""Opt-in SSH handshake diagnostics for explicitly allowlisted IP addresses.

This executor does not load Ansible inventory, Git files, or user commands.
It invokes exactly one non-interactive remote command, 'true', via an SSH key
restricted to a dedicated unprivileged account. Not a general Ansible runner.
"""
import fcntl
import ipaddress
import os
from pathlib import Path
import re
import resource
import signal
import stat
import subprocess
import threading
import time
from datetime import datetime, timezone

from fastapi import HTTPException
from sqlalchemy import select

from .database import DATA_DIR, SessionLocal, engine
from .models import SSHDiagnosticJob

SSHDiagnosticJob.__table__.create(bind=engine, checkfirst=True)

LOCK_PATH = DATA_DIR / "ssh-diagnostic-worker.lock"
ALIAS = re.compile(r"[a-z][a-z0-9_-]{1,39}\Z")
SSH_USER = "sakuraops-diag"
ACTIVE = ("queued", "running")
FINISHED = ("success", "failed", "cancelled")
TIMEOUT_SECONDS = 20
MAX_LOG = 16384
STOP = threading.Event()
WORKER = None


def _now() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


def _targets_from_config() -> dict[str, str]:
    """Aliases resolve to explicit numeric IPs, never DNS or inventory vars."""
    value = os.getenv("SAKURAOPS_SSH_DIAG_TARGETS", "")
    results: dict[str, str] = {}
    if not value.strip():
        return results
    for item in value.split(","):
        alias, separator, address = item.strip().partition("=")
        if not separator or not ALIAS.fullmatch(alias) or alias in results:
            raise HTTPException(503, "Nieprawidłowa lista hostów diagnostycznych")
        try:
            ip = ipaddress.ip_address(address.strip())
        except ValueError as exc:
            raise HTTPException(503, "Dozwolone są tylko jawne adresy IP") from exc
        if ip.is_multicast or ip.is_unspecified or ip.is_loopback or ip.is_link_local:
            raise HTTPException(503, "Niedozwolony adres diagnostyczny")
        results[alias] = str(ip)
    if len(results) > 20:
        raise HTTPException(503, "Maksymalnie 20 hostów diagnostycznych")
    return results


def _owned_regular_file(raw: str, *, private: bool) -> Path | None:
    if not raw or not Path(raw).is_absolute():
        return None
    path = Path(raw)
    try:
        mode = path.lstat()
    except OSError:
        return None
    if not stat.S_ISREG(mode.st_mode) or mode.st_uid != os.geteuid():
        return None
    if private and mode.st_mode & 0o077:
        return None
    if mode.st_mode & 0o022:
        return None
    return path


def _credentials() -> tuple[Path, Path] | None:
    key = _owned_regular_file(os.getenv("SAKURAOPS_SSH_DIAG_KEY", ""), private=True)
    hosts = _owned_regular_file(os.getenv("SAKURAOPS_SSH_DIAG_KNOWN_HOSTS", ""), private=False)
    if not key or not hosts:
        return None
    return key, hosts


def available() -> bool:
    return (os.getenv("SAKURAOPS_SSH_DIAG_ENABLED", "").lower() == "true"
            and bool(_targets_from_config()) and _credentials() is not None)


def configuration() -> dict:
    targets = _targets_from_config()
    valid = _credentials() is not None
    return {
        "enabled": available(),
        "account": SSH_USER,
        "targets": [{"alias": name, "address": address}
                    for name, address in sorted(targets.items())],
        "credentials_ready": valid,
    }


def _dto(job: SSHDiagnosticJob, include_log: bool = False) -> dict:
    row = {"id": job.id, "type": "ssh_connectivity", "alias": job.alias,
           "address": job.address, "status": job.status,
           "created_at": job.created_at, "started_at": job.started_at,
           "finished_at": job.finished_at, "exit_code": job.exit_code,
           "cancel_requested": job.cancel_requested}
    if include_log:
        row["log"] = job.log
    return row


def enqueue(alias: str) -> dict:
    if not available():
        raise HTTPException(409, "Diagnostyka SSH jest wyłączona lub nie jest skonfigurowana")
    targets = _targets_from_config()
    if alias not in targets:
        raise HTTPException(403, "Host nie znajduje się na liście dozwolonych")
    with SessionLocal() as db:
        active = db.scalars(select(SSHDiagnosticJob).where(
            SSHDiagnosticJob.status.in_(ACTIVE))).all()
        if len(active) >= 10:
            raise HTTPException(429, "Kolejka diagnostyczna jest pełna")
        job = SSHDiagnosticJob(alias=alias, address=targets[alias],
                               status="queued", created_at=_now(), log="")
        db.add(job)
        db.commit()
        db.refresh(job)
        return _dto(job)


def list_jobs() -> list[dict]:
    with SessionLocal() as db:
        return [_dto(job) for job in db.scalars(
            select(SSHDiagnosticJob).order_by(SSHDiagnosticJob.id.desc()).limit(50))]


def get_job(job_id: int) -> dict:
    with SessionLocal() as db:
        job = db.get(SSHDiagnosticJob, job_id)
        if job is None:
            raise HTTPException(404, "Nie znaleziono diagnostyki")
        return _dto(job, include_log=True)


def cancel_job(job_id: int) -> dict:
    with SessionLocal() as db:
        job = db.get(SSHDiagnosticJob, job_id)
        if job is None:
            raise HTTPException(404, "Nie znaleziono diagnostyki")
        if job.status in FINISHED:
            return _dto(job)
        job.cancel_requested = True
        if job.status == "queued":
            job.status = "cancelled"
            job.finished_at = _now()
            job.log = "[SakuraOps] Anulowano przed rozpoczęciem.\n"
        db.commit()
        db.refresh(job)
        return _dto(job)


def _append(job_id: int, text: str) -> None:
    with SessionLocal() as db:
        job = db.get(SSHDiagnosticJob, job_id)
        if job and job.status == "running":
            job.log = (job.log + text)[-MAX_LOG:]
            db.commit()


def _limits() -> None:
    resource.setrlimit(resource.RLIMIT_CPU, (10, 12))
    resource.setrlimit(resource.RLIMIT_FSIZE, (1024 * 1024, 1024 * 1024))


def _run(job_id: int) -> None:
    with SessionLocal() as db:
        job = db.get(SSHDiagnosticJob, job_id)
        if job is None:
            return
        alias, address = job.alias, job.address

    # Re-evaluate the allowlist at execution time, not just at enqueue.
    allowed = _targets_from_config()
    credentials = _credentials()
    if (not available() or allowed.get(alias) != address or credentials is None):
        _append(job_id, "[SakuraOps] Odmowa: host lub uprawnienia uległy zmianie.\n")
        outcome, code = "failed", None
    else:
        key, known_hosts = credentials
        cmd = [
            "/usr/bin/ssh", "-F", "/dev/null", "-T", "-n",
            "-o", "BatchMode=yes",
            "-o", "StrictHostKeyChecking=yes",
            "-o", "IdentitiesOnly=yes",
            "-o", "IdentityAgent=none",
            "-o", "ForwardAgent=no",
            "-o", "ForwardX11=no",
            "-o", "PermitLocalCommand=no",
            "-o", "ClearAllForwardings=yes",
            "-o", "ControlMaster=no",
            "-o", "ConnectTimeout=5",
            "-o", "ConnectionAttempts=1",
            "-o", "UserKnownHostsFile=" + str(known_hosts),
            "-i", str(key),
            "-l", SSH_USER,
            "--", address, "true",
        ]
        env = {"PATH": "/usr/bin:/bin", "HOME": "/nonexistent", "LANG": "C.UTF-8"}
        _append(job_id, "[SakuraOps] SSH handshake: " + alias
                + " (" + address + "), konto " + SSH_USER + ".\n")
        proc = None
        code = None
        outcome = "failed"
        try:
            proc = subprocess.Popen(cmd, stdin=subprocess.DEVNULL,
                                    stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                    env=env, start_new_session=True, preexec_fn=_limits)
            deadline = time.monotonic() + TIMEOUT_SECONDS
            while proc.poll() is None:
                with SessionLocal() as db:
                    current = db.get(SSHDiagnosticJob, job_id)
                    cancelled = current is None or current.cancel_requested
                if cancelled or STOP.is_set() or time.monotonic() > deadline:
                    os.killpg(proc.pid, signal.SIGKILL)
                    proc.wait(timeout=2)
                    outcome = "cancelled" if cancelled else "failed"
                    _append(job_id, "[SakuraOps] Anulowano lub przekroczono limit.\n")
                    break
                time.sleep(0.1)
            else:
                code = proc.returncode
                outcome = "success" if code == 0 else "failed"
            output = proc.stdout.read(MAX_LOG + 1) if proc.stdout else b""
            if output:
                _append(job_id, output[:MAX_LOG].decode("utf-8", "replace"))
            if code is not None:
                _append(job_id, "[SakuraOps] SSH exit code: " + str(code) + "\n")
        except OSError:
            _append(job_id, "[SakuraOps] Nie można uruchomić programu SSH.\n")
        finally:
            if proc and proc.poll() is None:
                os.killpg(proc.pid, signal.SIGKILL)
                proc.wait(timeout=2)
            if proc and proc.stdout:
                proc.stdout.close()
    with SessionLocal() as db:
        job = db.get(SSHDiagnosticJob, job_id)
        if job:
            job.status = "cancelled" if job.cancel_requested else outcome
            job.exit_code = code
            job.finished_at = _now()
            db.commit()


def _worker() -> None:
    fd = os.open(LOCK_PATH, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            return
        with SessionLocal() as db:
            for job in db.scalars(select(SSHDiagnosticJob).where(
                    SSHDiagnosticJob.status == "running")):
                job.status = "failed"
                job.finished_at = _now()
                job.log = (job.log + "\n[SakuraOps] Wykonawca został zrestartowany.\n")[-MAX_LOG:]
            db.commit()
        while not STOP.is_set():
            with SessionLocal() as db:
                job = db.scalar(select(SSHDiagnosticJob).where(
                    SSHDiagnosticJob.status == "queued").order_by(SSHDiagnosticJob.id))
                if job:
                    job.status = "running"
                    job.started_at = _now()
                    job_id = job.id
                    db.commit()
                else:
                    job_id = None
            if job_id is None:
                STOP.wait(0.3)
                continue
            try:
                _run(job_id)
            except Exception:
                with SessionLocal() as db:
                    job = db.get(SSHDiagnosticJob, job_id)
                    if job:
                        job.status = "failed"
                        job.finished_at = _now()
                        job.log = (job.log + "\n[SakuraOps] Błąd diagnostyki.\n")[-MAX_LOG:]
                        db.commit()
    finally:
        os.close(fd)


def start_worker() -> None:
    global WORKER
    if not available():
        return
    if WORKER and WORKER.is_alive():
        return
    STOP.clear()
    WORKER = threading.Thread(target=_worker, daemon=True, name="sakura-ssh-diagnostic")
    WORKER.start()


def stop_worker() -> None:
    STOP.set()
    if WORKER and WORKER.is_alive():
        WORKER.join(timeout=3)

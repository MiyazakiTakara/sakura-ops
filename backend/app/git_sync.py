"""Manual, read-only Git fetch using a pinned SSH identity and host key.

This module never checks out files, executes Ansible, or uses credentials
provided in a command. It operates on bare Git repositories only.
"""
import fcntl
import os
import re
import shlex
import signal
import subprocess
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlsplit

from fastapi import HTTPException
from sqlalchemy import select

from .database import DATA_DIR, SessionLocal, engine
from .models import GitRepository, GitSyncState

GitSyncState.__table__.create(bind=engine, checkfirst=True)
MIRRORS = DATA_DIR / "git_mirrors"
MIRRORS.mkdir(parents=True, exist_ok=True, mode=0o700)
SHA = re.compile(r"[0-9a-f]{40}\Z")
REF = "refs/sakuraops/synced"


def _settings(url: str) -> tuple[str, list[str]]:
    parsed = urlsplit(url)
    if parsed.scheme != "ssh" or parsed.username != "git" or not parsed.hostname:
        raise HTTPException(422, "Synchronizacja wymaga adresu ssh://git@host/owner/repo.git")
    if parsed.password or parsed.query or parsed.fragment or parsed.port not in (None, 22, 2222):
        raise HTTPException(422, "Nieprawidłowy SSH URL")
    hosts = {x.strip().lower() for x in os.getenv("SAKURAOPS_GIT_ALLOWED_HOSTS", "").split(",") if x.strip()}
    if parsed.hostname.lower() not in hosts:
        raise HTTPException(403, "Host Git nie jest dopuszczony do synchronizacji")
    key = Path(os.getenv("SAKURAOPS_GIT_SSH_KEY", ""))
    known = Path(os.getenv("SAKURAOPS_GIT_KNOWN_HOSTS", ""))
    if not key.is_file() or key.is_symlink() or not known.is_file() or known.is_symlink():
        raise HTTPException(409, "Skonfiguruj klucz deploy SSH i zweryfikowany known_hosts")
    if key.stat().st_mode & 0o077:
        raise HTTPException(409, "Klucz prywatny SSH musi mieć uprawnienia 0600")
    ssh = [
        "ssh", "-F", "/dev/null", "-i", str(key.resolve()),
        "-o", "IdentitiesOnly=yes", "-o", "BatchMode=yes",
        "-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile=" + str(known.resolve()),
        "-o", "GlobalKnownHostsFile=/dev/null", "-o", "ProxyCommand=none",
        "-o", "ProxyJump=none", "-o", "ForwardAgent=no",
        "-o", "ConnectTimeout=8",
    ]
    return parsed.hostname, ssh


def _git(args: list[str], ssh: list[str] | None = None) -> str:
    env = dict(os.environ)
    env["GIT_CONFIG_NOSYSTEM"] = "1"
    env["GIT_CONFIG_GLOBAL"] = os.devnull
    env["GIT_TERMINAL_PROMPT"] = "0"
    env["GIT_ALLOW_PROTOCOL"] = "ssh"
    if ssh:
        env["GIT_SSH_COMMAND"] = shlex.join(ssh)
    cmd = ["git", "-c", "core.hooksPath=/dev/null", *args]
    process = subprocess.Popen(
        cmd, env=env, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
        text=True, start_new_session=True,
    )
    try:
        stdout, _ = process.communicate(timeout=50)
    except subprocess.TimeoutExpired as exc:
        os.killpg(process.pid, signal.SIGKILL)
        process.communicate()
        raise HTTPException(504, "Przekroczono limit czasu synchronizacji Git") from exc
    if process.returncode:
        raise HTTPException(502, "Operacja Git nie powiodła się (sprawdź SSH, gałąź i dostęp)")
    return stdout.strip()


def _mirror(name: str) -> Path:
    if not re.fullmatch(r"[a-z][a-z0-9_-]{1,39}", name):
        raise HTTPException(400, "Nieprawidłowa nazwa repozytorium")
    return MIRRORS / (name + ".git")


def _cached_sha(name: str) -> str | None:
    with SessionLocal() as db:
        row = db.scalar(select(GitSyncState).where(GitSyncState.repository_name == name))
        return row.commit_sha if row is not None else None


def synchronization_status(name: str) -> dict:
    with SessionLocal() as db:
        row = db.scalar(select(GitSyncState).where(GitSyncState.repository_name == name))
        if row is None:
            return {"sync_status": "not_synced", "commit_sha": None, "synced_at": None}
        return {"sync_status": "synced", "commit_sha": row.commit_sha, "synced_at": row.synced_at}


def synchronize(name: str) -> dict:
    with SessionLocal() as db:
        repo = db.scalar(select(GitRepository).where(GitRepository.name == name))
        if repo is None:
            raise HTTPException(404, "Nie znaleziono repozytorium")
        url, branch = repo.url, repo.branch
    _, ssh = _settings(url)
    mirror = _mirror(name)
    lock_path = MIRRORS / (name + ".lock")
    lock_fd = os.open(lock_path, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        try:
            fcntl.flock(lock_fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            raise HTTPException(409, "Synchronizacja repozytorium już trwa") from exc
        if mirror.is_symlink():
            raise HTTPException(409, "Nieprawidłowa ścieżka repozytorium")
        if not mirror.exists():
            mirror.mkdir(mode=0o700)
            _git(["init", "--bare", str(mirror)])
        if not mirror.is_dir():
            raise HTTPException(409, "Nieprawidłowy katalog repozytorium")
        # No shell expansion, no checkout, no submodules and no git hooks.
        _git(["-C", str(mirror), "fetch", "--no-tags", "--depth=1",
              "--", url, branch], ssh)
        commit_sha = _git(["-C", str(mirror), "rev-parse", "--verify", "FETCH_HEAD"])
        if not SHA.fullmatch(commit_sha):
            raise HTTPException(502, "Nieprawidłowy identyfikator Git")
        _git(["-C", str(mirror), "update-ref", REF, commit_sha])
        stamp = datetime.now(timezone.utc).isoformat(timespec="seconds")
        with SessionLocal() as db:
            state = db.scalar(select(GitSyncState).where(GitSyncState.repository_name == name))
            if state is None:
                state = GitSyncState(repository_name=name, commit_sha=commit_sha, synced_at=stamp)
                db.add(state)
            else:
                state.commit_sha = commit_sha
                state.synced_at = stamp
            db.commit()
        return synchronization_status(name)
    finally:
        os.close(lock_fd)


def list_cached_files(name: str, kind: str) -> list[str]:
    if kind not in ("inventory", "playbooks"):
        raise HTTPException(422, "Dozwolone rodzaje: inventory, playbooks")
    with SessionLocal() as db:
        repo = db.scalar(select(GitRepository).where(GitRepository.name == name))
        if repo is None:
            raise HTTPException(404, "Nie znaleziono repozytorium")
    commit_sha = _cached_sha(name)
    if not commit_sha:
        raise HTTPException(409, "Najpierw wykonaj synchronizację Git")
    mirror = _mirror(name)
    if mirror.is_symlink() or not mirror.is_dir():
        raise HTTPException(409, "Nie znaleziono lokalnego cache Git")
    output = _git(["-C", str(mirror), "ls-tree", "-r", "--name-only", commit_sha])
    folders = ("inventory/", "inventories/") if kind == "inventory" else ("playbook/", "playbooks/")
    return sorted(
        x for x in output.splitlines()
        if x.startswith(folders) and x.endswith((".ini", ".yml", ".yaml"))
        and ".." not in x.split("/")
    )[:200]

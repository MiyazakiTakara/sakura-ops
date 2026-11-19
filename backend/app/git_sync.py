"""Synchronizacja normalnych katalogów roboczych Git dla SakuraOps.

Korzysta z konfiguracji SSH, credential helperów i CA użytkownika usługi.
Nigdy nie wykonuje Ansible, git push ani poleceń powłoki dostarczonych przez UI.
"""
import fcntl
import os
import re
import shutil
import subprocess
import tempfile
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlsplit

from fastapi import HTTPException
from sqlalchemy import select

from .database import DATA_DIR, SessionLocal, engine
from .models import GitRepository, GitSyncState

GitSyncState.__table__.create(bind=engine, checkfirst=True)
WORKTREES = DATA_DIR / "git_worktrees"
WORKTREES.mkdir(parents=True, exist_ok=True, mode=0o700)
NAME = re.compile(r"[a-z][a-z0-9_-]{1,39}\Z")
SHA = re.compile(r"[0-9a-f]{40}\Z")


def _location(name: str) -> Path:
    if not NAME.fullmatch(name):
        raise HTTPException(400, "Nieprawidłowa nazwa repozytorium")
    return WORKTREES / name


def _check_remote(url: str) -> None:
    """Reject local protocols and optionally enforce an administrator host allowlist."""
    try:
        remote = urlsplit(url)
        host = remote.hostname
        port = remote.port
    except ValueError as exc:
        raise HTTPException(422, "Nieprawidłowy adres Git") from exc
    if (remote.scheme not in ("https", "ssh") or not host or
        remote.password or remote.query or remote.fragment or
        (remote.scheme == "https" and remote.username) or
        (remote.scheme == "ssh" and remote.username not in (None, "git")) or
        (remote.scheme == "https" and port not in (None, 443)) or
        (remote.scheme == "ssh" and port not in (None, 22, 2222)) or
        not remote.path or remote.path == "/"):
        raise HTTPException(422, "Wymagany adres HTTPS lub ssh://git@host/repo")
    allowed = {host.strip().lower() for host in
               os.getenv("SAKURAOPS_GIT_ALLOWED_HOSTS", "").split(",") if host.strip()}
    if allowed and host.lower() not in allowed:
        raise HTTPException(403, "Host Git nie znajduje się na liście dozwolonych")


def _git(args: list[str], timeout: int = 90) -> str:
    """Normal Git with current service user's credentials, noninteractive in web calls."""
    env = dict(os.environ)
    for key in list(env):
        if key.startswith(("GIT_CONFIG_", "GIT_ASKPASS", "GIT_SSH", "SSH_ASKPASS")) or key in (
            "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_CEILING_DIRECTORIES",
            "GIT_TEMPLATE_DIR", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
        ):
            env.pop(key, None)
    env["GIT_TERMINAL_PROMPT"] = "0"
    env["GIT_ALLOW_PROTOCOL"] = "https:ssh"
    env["GIT_LFS_SKIP_SMUDGE"] = "1"
    command = [
        "git", "-c", "core.hooksPath=/dev/null",
        "-c", "protocol.file.allow=never", "-c", "protocol.ext.allow=never",
        "-c", "fetch.recurseSubmodules=false",
        *args,
    ]
    try:
        run = subprocess.run(command, env=env, input="", text=True,
                             stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                             timeout=timeout, check=False)
    except subprocess.TimeoutExpired as exc:
        raise HTTPException(504, "Przekroczony czas wykonania Git") from exc
    except OSError as exc:
        raise HTTPException(500, "Program Git jest niedostępny") from exc
    if run.returncode:
        raise HTTPException(502, "Operacja Git nie powiodła się. Sprawdź dostęp, gałąź i konfigurację Git użytkownika usługi")
    return run.stdout.strip()


def _config(name: str) -> tuple[str, str]:
    with SessionLocal() as db:
        repo = db.scalar(select(GitRepository).where(GitRepository.name == name))
        if repo is None:
            raise HTTPException(404, "Nie znaleziono repozytorium")
        return repo.url, repo.branch


def synchronization_status(name: str) -> dict:
    with SessionLocal() as db:
        item = db.scalar(select(GitSyncState).where(GitSyncState.repository_name == name))
        if item is None:
            return {"sync_status": "not_synced", "commit_sha": None, "synced_at": None}
        return {"sync_status": "synced", "commit_sha": item.commit_sha, "synced_at": item.synced_at}


def synchronize(name: str) -> dict:
    url, branch = _config(name)
    _check_remote(url)
    target = _location(name)
    lock = WORKTREES / (name + ".lock")
    fd = os.open(lock, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            raise HTTPException(409, "Synchronizacja już trwa") from exc

        if target.is_symlink():
            raise HTTPException(409, "Nieprawidłowy katalog Git")
        if not target.exists():
            temporary = Path(tempfile.mkdtemp(prefix=".git-import-", dir=WORKTREES))
            try:
                checkout = temporary / "checkout"
                _git(["clone", "--single-branch", "--no-tags", "--no-recurse-submodules",
                      "--branch", branch, "--", url, str(checkout)])
                sha = _git(["-C", str(checkout), "rev-parse", "--verify", "HEAD"])
                if not SHA.fullmatch(sha):
                    raise HTTPException(502, "Nieprawidłowy commit Git")
                if target.exists():
                    raise HTTPException(409, "Katalog repozytorium już istnieje")
                checkout.rename(target)
            finally:
                shutil.rmtree(temporary, ignore_errors=True)
        else:
            if not target.is_dir() or not (target / ".git").is_dir():
                raise HTTPException(409, "Katalog nie jest zwykłym repozytorium Git")
            if _git(["-C", str(target), "config", "--get", "remote.origin.url"]) != url:
                raise HTTPException(409, "Adres origin różni się od konfiguracji SakuraOps")
            if _git(["-C", str(target), "branch", "--show-current"]) != branch:
                raise HTTPException(409, "Gałąź robocza nie zgadza się z konfiguracją")
            if _git(["-C", str(target), "status", "--porcelain", "--untracked-files=all"]):
                raise HTTPException(409, "Lokalne zmiany Git: synchronizacja nie nadpisze plików")
            _git(["-C", str(target), "fetch", "--no-tags", "--no-recurse-submodules",
                  "--", "origin", branch])
            _git(["-C", str(target), "merge", "--ff-only", "FETCH_HEAD"])
            sha = _git(["-C", str(target), "rev-parse", "--verify", "HEAD"])
            if not SHA.fullmatch(sha):
                raise HTTPException(502, "Nieprawidłowy commit Git")

        stamp = datetime.now(timezone.utc).isoformat(timespec="seconds")
        with SessionLocal() as db:
            row = db.scalar(select(GitSyncState).where(GitSyncState.repository_name == name))
            if row is None:
                db.add(GitSyncState(repository_name=name, commit_sha=sha, synced_at=stamp))
            else:
                row.commit_sha = sha
                row.synced_at = stamp
            db.commit()
        return synchronization_status(name)
    finally:
        os.close(fd)


def list_cached_files(name: str, kind: str) -> list[str]:
    if kind not in ("inventory", "playbooks"):
        raise HTTPException(422, "Wybierz inventory albo playbooks")
    _config(name)
    target = _location(name)
    if target.is_symlink() or not target.is_dir() or not (target / ".git").is_dir():
        raise HTTPException(409, "Najpierw zsynchronizuj repozytorium Git")
    paths = ("inventory", "inventories") if kind == "inventory" else ("playbook", "playbooks")
    output = _git(["-C", str(target), "ls-files", "--", *paths])
    return sorted(
        line for line in output.splitlines()
        if line.endswith((".ini", ".yaml", ".yml")) and
        len(line) <= 500 and ".." not in line.split("/")
    )[:200]

"""Controlled Git status, selected-file commits and explicit non-force pushes.

Only tracked inventory and playbook files may be committed. No arbitrary Git
arguments, shell execution, auto-push or playbook execution.
"""
from contextlib import contextmanager
import fcntl
import os
import re
from pathlib import Path

from fastapi import HTTPException

from . import git_sync

COMMIT_SHA = re.compile(r"[0-9a-f]{40}\Z")
ALLOWED_MESSAGE_LENGTH = 200


@contextmanager
def _locked(name: str):
    lock = git_sync.WORKTREES / (name + ".lock")
    fd = os.open(lock, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            raise HTTPException(409, "Inna operacja Git jest w toku") from exc
        yield
    finally:
        os.close(fd)


def _checked_worktree(name: str) -> tuple[Path, str]:
    url, branch = git_sync._config(name)
    git_sync._check_remote(url)
    target = git_sync._location(name)
    if target.is_symlink() or not target.is_dir() or not (target / ".git").is_dir():
        raise HTTPException(409, "Najpierw zsynchronizuj repozytorium")
    if git_sync._git(["-C", str(target), "config", "--get", "remote.origin.url"]) != url:
        raise HTTPException(409, "Adres origin nie jest zgodny z konfiguracją")
    if git_sync._git(["-C", str(target), "branch", "--show-current"]) != branch:
        raise HTTPException(409, "Bieżąca gałąź jest inna niż skonfigurowana")
    return target, branch


def _candidate_paths(name: str) -> set[str]:
    return set(git_sync.list_cached_files(name, "inventory")) | set(
        git_sync.list_cached_files(name, "playbooks")
    )


def _modified_paths(name: str, target: Path) -> list[str]:
    # NUL separator disallows ambiguous whitespace and filenames with newlines.
    output = git_sync._git([
        "-C", str(target), "diff", "--name-only", "-z",
        "--diff-filter=M", "HEAD", "--",
        "inventory", "inventories", "playbook", "playbooks",
    ])
    allowed = _candidate_paths(name)
    paths = [x for x in output.split("\x00") if x]
    return sorted(x for x in paths if x in allowed and "\n" not in x and "\r" not in x)


def git_status(name: str) -> dict:
    target, branch = _checked_worktree(name)
    sha = git_sync._git(["-C", str(target), "rev-parse", "--verify", "HEAD"])
    files = _modified_paths(name, target)
    status = git_sync._git(["-C", str(target), "status", "--porcelain", "--untracked-files=all"])
    return {
        "repository": name, "branch": branch, "head": sha,
        "modified_files": files, "dirty": bool(status),
        "other_changes": bool(status) and not files,
    }


def commit_selected(name: str, files: list[str], message: str, expected_head: str) -> dict:
    message = message.strip()
    if not message or len(message) > ALLOWED_MESSAGE_LENGTH or any(
        ord(ch) < 32 and ch != "\t" for ch in message
    ):
        raise HTTPException(422, "Opis commita musi mieć 1–200 znaków bez znaków sterujących")
    if not COMMIT_SHA.fullmatch(expected_head):
        raise HTTPException(422, "Wymagany poprawny aktualny commit")
    if not 1 <= len(files) <= 30 or len(set(files)) != len(files):
        raise HTTPException(422, "Wybierz od 1 do 30 różnych plików")
    with _locked(name):
        target, _ = _checked_worktree(name)
        head = git_sync._git(["-C", str(target), "rev-parse", "--verify", "HEAD"])
        if head != expected_head:
            raise HTTPException(409, "Zmienił się commit HEAD, odśwież status")
        permitted = set(_modified_paths(name, target))
        if not set(files) <= permitted:
            raise HTTPException(409, "Wybrane pliki zostały zmienione albo nie są dozwolone")
        for filename in files:
            git_sync._editable_path(name, filename)
        # --only commits working-tree versions of explicitly selected tracked files,
        # without including unrelated entries already staged in the index.
        git_sync._git([
            "-C", str(target), "-c", "commit.gpgSign=false", "commit",
            "--only", "-m", message, "--", *files,
        ])
        new_head = git_sync._git(["-C", str(target), "rev-parse", "--verify", "HEAD"])
        return {"repository": name, "commit_sha": new_head, "files": files}


def push_explicit(name: str, expected_head: str) -> dict:
    if not COMMIT_SHA.fullmatch(expected_head):
        raise HTTPException(422, "Podaj pełne 40 znaków SHA do potwierdzenia")
    with _locked(name):
        target, branch = _checked_worktree(name)
        head = git_sync._git(["-C", str(target), "rev-parse", "--verify", "HEAD"])
        if head != expected_head:
            raise HTTPException(409, "HEAD zmienił się od potwierdzenia. Odśwież status")
        # A push is never forced and never starts from a periodic synchronizer.
        # A remote update will result in Git's default non-fast-forward rejection.
        git_sync._git(["-C", str(target), "push", "--porcelain",
                       "origin", "HEAD:refs/heads/" + branch], timeout=90)
        return {"repository": name, "pushed_sha": head, "branch": branch}

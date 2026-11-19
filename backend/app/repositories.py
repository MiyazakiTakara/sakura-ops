"""Shared Git repository configuration for the GUI and SakuraOps console.

Only metadata is stored. No clone/fetch, credentials or repository execution.
"""
import re
from urllib.parse import urlsplit

from fastapi import HTTPException
from sqlalchemy import select
from sqlalchemy.exc import IntegrityError

from .database import SessionLocal, engine
from .models import GitRepository

GitRepository.__table__.create(bind=engine, checkfirst=True)

NAME_PATTERN = re.compile(r"[a-z][a-z0-9_-]{1,39}\Z")
BRANCH_PATTERN = re.compile(r"[A-Za-z0-9][A-Za-z0-9_./-]{0,99}\Z")


def validate(name: str, url: str, branch: str) -> tuple[str, str, str]:
    name, url, branch = name.strip(), url.strip(), branch.strip()
    if not NAME_PATTERN.fullmatch(name):
        raise HTTPException(422, "Nazwa: 2-40 znaków a-z, cyfr, _ lub -")
    if not BRANCH_PATTERN.fullmatch(branch) or ".." in branch or branch.endswith(("/", ".")) or "//" in branch:
        raise HTTPException(422, "Nieprawidłowa nazwa gałęzi")
    if len(url) > 500 or any(ord(c) < 33 or ord(c) > 126 for c in url):
        raise HTTPException(422, "Nieprawidłowy adres repozytorium")
    parsed = urlsplit(url)
    if (parsed.scheme not in ("https", "ssh") or not parsed.hostname or parsed.password or
        parsed.username and parsed.scheme == "https" or parsed.query or parsed.fragment or
        parsed.port not in (None, 22, 443, 80, 2222)):
        raise HTTPException(422, "Podaj adres HTTPS lub SSH bez hasła, tokenu i parametrów")
    if parsed.scheme == "ssh" and parsed.username not in (None, "git"):
        raise HTTPException(422, "Adres SSH może używać wyłącznie użytkownika git")
    if not parsed.path or parsed.path == "/":
        raise HTTPException(422, "Adres repozytorium musi zawierać ścieżkę")
    return name, url, branch


def dto(repo: GitRepository) -> dict:
    return {"name": repo.name, "url": repo.url, "branch": repo.branch,
            "status": "configured", "sync_enabled": False}


def list_repositories() -> list[dict]:
    with SessionLocal() as db:
        return [dto(row) for row in db.scalars(select(GitRepository).order_by(GitRepository.name))]


def add_repository(name: str, url: str, branch: str = "main") -> dict:
    name, url, branch = validate(name, url, branch)
    with SessionLocal() as db:
        repo = GitRepository(name=name, url=url, branch=branch)
        db.add(repo)
        try:
            db.commit()
        except IntegrityError as exc:
            db.rollback()
            raise HTTPException(409, "Repozytorium o tej nazwie już istnieje") from exc
        db.refresh(repo)
        return dto(repo)


def delete_repository(name: str) -> None:
    with SessionLocal() as db:
        repo = db.scalar(select(GitRepository).where(GitRepository.name == name))
        if repo is None:
            raise HTTPException(404, "Nie znaleziono repozytorium")
        db.delete(repo)
        db.commit()

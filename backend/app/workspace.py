"""Small, isolated text workspace. Not a filesystem browser or shell."""
import os
import re
import stat
from pathlib import Path

from fastapi import HTTPException

WORKSPACE = Path(__file__).resolve().parents[2] / "data" / "workspace"
WORKSPACE.mkdir(parents=True, exist_ok=True, mode=0o700)
SUFFIXES = {".txt", ".md", ".ini", ".yml", ".yaml"}
MAX_BYTES = 65536


def resolve_name(filename: str) -> Path:
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]{0,79}", filename):
        raise HTTPException(400, "Nieprawidłowa nazwa pliku")
    if filename in (".", "..") or Path(filename).suffix.lower() not in SUFFIXES:
        raise HTTPException(400, "Dozwolone są pliki TXT, MD, INI, YAML")
    return WORKSPACE / filename


def list_files() -> list[str]:
    return sorted(
        file.name for file in WORKSPACE.iterdir()
        if file.is_file() and not file.is_symlink()
        and file.name[0].isalnum() and file.suffix.lower() in SUFFIXES
    )


def read_text(filename: str) -> str:
    path = resolve_name(filename)
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    except FileNotFoundError:
        return ""
    except OSError as exc:
        raise HTTPException(400, "Nie można otworzyć pliku") from exc
    try:
        if not stat.S_ISREG(os.fstat(fd).st_mode):
            raise HTTPException(400, "To nie jest zwykły plik")
        with os.fdopen(fd, "rb", closefd=False) as stream:
            content = stream.read(MAX_BYTES + 1)
        if len(content) > MAX_BYTES:
            raise HTTPException(413, "Plik przekracza 64 KiB")
        return content.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise HTTPException(400, "Plik nie jest w UTF-8") from exc
    finally:
        os.close(fd)


def write_text(filename: str, content: str) -> None:
    path = resolve_name(filename)
    data = content.encode("utf-8")
    if len(data) > MAX_BYTES:
        raise HTTPException(413, "Maksymalny rozmiar pliku: 64 KiB")
    try:
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    except OSError as exc:
        raise HTTPException(400, "Nie można zapisać pliku") from exc
    try:
        if not stat.S_ISREG(os.fstat(fd).st_mode):
            raise HTTPException(400, "To nie jest zwykły plik")
        os.ftruncate(fd, 0)
        with os.fdopen(fd, "wb", closefd=False) as stream:
            stream.write(data)
    finally:
        os.close(fd)

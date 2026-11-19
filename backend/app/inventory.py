"""Read-only discovery of Ansible files in an explicitly configured Git checkout.

SakuraOps does not clone, pull, execute or interpret inventory here.
Only relative file names under the checkout are returned; file contents,
credentials and host variables are not exposed.
"""
import os
from pathlib import Path

ALLOWED = {".yml", ".yaml", ".ini"}
MAX_FILES = 200


def checkout_root() -> Path | None:
    raw = os.getenv("SAKURAOPS_ANSIBLE_REPO_PATH", "")
    if not raw:
        return None
    root = Path(raw).expanduser().resolve()
    # Refuse arbitrary paths until the operator configures a Git worktree.
    if not root.is_dir() or not (root / ".git").exists():
        return None
    return root


def discover(kind: str) -> tuple[str, list[str]]:
    """List Ansible-related files; caller decides if user may access it."""
    root = checkout_root()
    if root is None:
        return (
            "Repozytorium Ansible nie jest skonfigurowane. "
            "Ustaw SAKURAOPS_ANSIBLE_REPO_PATH na lokalny checkout Git.",
            [],
        )
    folders = ("inventory", "inventories") if kind == "inventory" else ("playbooks", "playbook")
    result: list[str] = []
    for folder in folders:
        base = root / folder
        if not base.is_dir() or base.is_symlink():
            continue
        for path in base.rglob("*"):
            if len(result) >= MAX_FILES:
                break
            if path.is_symlink() or not path.is_file() or path.suffix.lower() not in ALLOWED:
                continue
            relative = path.relative_to(root).as_posix()
            result.append(relative)
    if not result:
        return ("Nie znaleziono plików w katalogach " + ", ".join(folders) + ".", [])
    return ("Pliki repozytorium (tylko odczyt, bez wykonywania):", sorted(result))

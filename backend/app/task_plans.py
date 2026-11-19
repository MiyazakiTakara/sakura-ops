"""Read-only preflight of Ansible task targets. Never starts a subprocess."""
import re

from fastapi import HTTPException

from .git_sync import list_cached_files, read_repository_file
from .inventory_preview import preview_inventory

GROUP = re.compile(r"[A-Za-z0-9_.:@-]{1,255}\Z")


def task_plan(repository: str, inventory: str, playbook: str, group: str) -> dict:
    """Validate a single static group target and a tracked playbook.

    This is deliberately *not* an authoritative Ansible execution plan:
    dynamic inventory and playbook conditions are not evaluated.
    """
    if not GROUP.fullmatch(group):
        raise HTTPException(422, "Podaj pojedynczą grupę inventory")
    if playbook not in list_cached_files(repository, "playbooks"):
        raise HTTPException(404, "Playbook nie jest śledzonym plikiem Git")
    # Validate paths, type, symlinks and size without exposing playbook content.
    read_repository_file(repository, playbook)
    source = preview_inventory(repository, inventory)
    target = next((item for item in source["groups"] if item["name"] == group), None)
    if target is None:
        raise HTTPException(422, "Wybrana grupa nie istnieje w inventory")
    if not target["hosts"]:
        raise HTTPException(422, "Wybrana grupa nie ma hostów")
    return {
        "repository": repository,
        "inventory": inventory,
        "playbook": playbook,
        "limit": group,
        "hosts": target["hosts"],
        "host_count": target["host_count"],
        "state": "preview_only",
        "executable": False,
        "warning": "Statyczny podgląd grupy, bez ewaluacji Ansible. Nic nie zostanie uruchomione.",
    }

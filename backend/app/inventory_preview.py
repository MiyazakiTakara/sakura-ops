"""Passive Ansible inventory preview from a tracked, synchronized Git file.

No ansible-inventory subprocess, Jinja, variable interpolation, plugin loading
or network activity. Values from host/group vars are deliberately never returned.
"""
import re

from fastapi import HTTPException

from .git_sync import read_repository_file, list_cached_files

IDENT = re.compile(r"[A-Za-z0-9_.:@-]{1,255}\Z")
LIMIT_HOSTS = 2000
LIMIT_GROUPS = 500


def _valid(value: object) -> bool:
    return isinstance(value, str) and bool(IDENT.fullmatch(value))


def _read_ini(content: str) -> tuple[dict[str, set[str]], dict[str, set[str]]]:
    groups: dict[str, set[str]] = {}
    children: dict[str, set[str]] = {}
    section = "ungrouped"
    section_kind = "hosts"
    for raw in content.splitlines():
        line = raw.strip()
        if not line or line.startswith(("#", ";")):
            continue
        if line.startswith("[") and line.endswith("]"):
            section = line[1:-1].strip()
            section_kind = "hosts"
            if section.endswith(":children"):
                section, section_kind = section[:-9], "children"
            elif section.endswith(":vars"):
                section, section_kind = section[:-5], "vars"
            if not _valid(section):
                raise HTTPException(422, "Nieprawidłowa nazwa grupy w inventory INI")
            if section_kind != "vars":
                groups.setdefault(section, set())
            continue
        if section_kind == "vars":
            continue
        if section_kind == "children":
            child = line.split("#", 1)[0].strip()
            if not _valid(child):
                raise HTTPException(422, "Nieprawidłowa nazwa grupy podrzędnej")
            children.setdefault(section, set()).add(child)
            groups.setdefault(child, set())
            continue
        # INI inventory: first whitespace-separated token is a host alias.
        host = line.split("#", 1)[0].split(maxsplit=1)[0] if line.split("#", 1)[0].strip() else ""
        if not _valid(host):
            raise HTTPException(422, "Nieprawidłowy host w inventory INI")
        groups.setdefault(section, set()).add(host)
    return groups, children


def _read_yaml(content: str) -> tuple[dict[str, set[str]], dict[str, set[str]]]:
    try:
        import yaml
    except ImportError as exc:
        raise HTTPException(503, "Obsługa YAML wymaga pakietu PyYAML") from exc
    try:
        tree = yaml.safe_load(content)
    except yaml.YAMLError as exc:
        raise HTTPException(422, "Nieprawidłowa składnia inventory YAML") from exc
    if not isinstance(tree, dict):
        raise HTTPException(422, "Inventory YAML musi zawierać mapę grup")
    groups: dict[str, set[str]] = {}
    children: dict[str, set[str]] = {}
    visited: set[int] = set()

    def visit(name: str, node: object, depth: int) -> None:
        if not _valid(name) or depth > 30:
            raise HTTPException(422, "Nieprawidłowa nazwa lub głębokość grupy YAML")
        if not isinstance(node, dict):
            raise HTTPException(422, "Grupa inventory YAML musi być mapą")
        # Prevent recursive YAML aliases and pathological nested structures.
        if id(node) in visited:
            raise HTTPException(422, "Powtarzające się lub cykliczne aliasy YAML")
        visited.add(id(node))
        groups.setdefault(name, set())
        hosts = node.get("hosts", {})
        if hosts is not None:
            if not isinstance(hosts, dict):
                raise HTTPException(422, "Pole hosts musi być mapą")
            for host in hosts:
                if not _valid(host):
                    raise HTTPException(422, "Nieprawidłowy host w inventory YAML")
                groups[name].add(host)
        nested = node.get("children", {})
        if nested is not None:
            if not isinstance(nested, dict):
                raise HTTPException(422, "Pole children musi być mapą")
            for child, subtree in nested.items():
                if not _valid(child):
                    raise HTTPException(422, "Nieprawidłowa grupa podrzędna")
                children.setdefault(name, set()).add(child)
                visit(child, subtree, depth + 1)
        if len(groups) > LIMIT_GROUPS or sum(map(len, groups.values())) > LIMIT_HOSTS * 2:
            raise HTTPException(413, "Inventory przekracza limit podglądu")

    for group, node in tree.items():
        visit(group, node, 0)
    return groups, children


def preview_inventory(name: str, filename: str) -> dict:
    if filename not in list_cached_files(name, "inventory"):
        raise HTTPException(404, "Plik inventory nie istnieje albo nie jest śledzony")
    content = read_repository_file(name, filename)["content"]
    if filename.lower().endswith(".ini"):
        groups, children = _read_ini(content)
        source = "static-ini"
    elif filename.lower().endswith((".yaml", ".yml")):
        groups, children = _read_yaml(content)
        source = "static-yaml"
    else:
        raise HTTPException(422, "Nieobsługiwane rozszerzenie inventory")

    if len(groups) > LIMIT_GROUPS:
        raise HTTPException(413, "Zbyt wiele grup")
    all_hosts = set().union(*groups.values()) if groups else set()
    if len(all_hosts) > LIMIT_HOSTS:
        raise HTTPException(413, "Zbyt wiele hostów")

    # Child groups contribute hosts to their parents, as in Ansible inventory.
    def resolved(group: str, stack: frozenset[str]) -> set[str]:
        if group in stack:
            raise HTTPException(422, "Cykl grup potomnych w inventory")
        values = set(groups.get(group, ()))
        for child in children.get(group, ()):
            values.update(resolved(child, stack | {group}))
        return values

    result = []
    for group in sorted(groups):
        direct = sorted(groups[group])
        total = sorted(resolved(group, frozenset()))
        result.append({
            "name": group, "hosts": total, "direct_hosts": direct,
            "children": sorted(children.get(group, ())), "host_count": len(total),
        })
    return {
        "repository": name, "path": filename, "parser": source,
        "hosts": sorted(all_hosts), "host_count": len(all_hosts),
        "groups": result, "group_count": len(result),
        "limitations": "Podgląd statyczny: bez pluginów, zmiennych, szablonów i wykonywania Ansible.",
    }

"""Context-aware, read-only Tab completion for the restricted SakuraOps CLI.

Every suggestion is a valid prefix assembled from registered commands and
current authorized configuration. No system shell, network calls, or secrets.
"""
import re

from fastapi import HTTPException

from .commands import complete as static_complete

# Text fields requiring manual input deliberately do not autocomplete.
GRAMMAR_EXEC = (
    ("show", "version"), ("show", "status"), ("show", "servers"),
    ("show", "inventory"), ("show", "playbooks"), ("show", "commands"),
    ("show", "repositories"),
    ("show", "hosts", "{repo}", "{inventory}"),
    ("show", "groups", "{repo}", "{inventory}"),
    ("show", "repository", "{repo}", "inventory"),
    ("show", "repository", "{repo}", "playbooks"),
    ("show", "repository", "{repo}", "file", "{allfiles}"),
    ("show", "repository", "{repo}", "diff", "{allfiles}"),
    ("task", "plan", "{repo}", "{inventory}", "{playbook}", "{group}"),
    ("repository", "sync", "{repo}"),
    ("repository", "status", "{repo}"),
    ("repository", "edit", "{repo}", "{allfiles}"),
    ("repository", "push", "{repo}", "{sha}"),
    ("repository", "commit", "{repo}", "{sha}", "{message}", "{file}"),
    ("configure", "terminal"),
    ("edit", "{workspace}"), ("nano", "{workspace}"), ("cat", "{workspace}"),
    ("files",), ("clear",), ("history",), ("exit",), ("end",), ("help",),
)
GRAMMAR_CONFIG = (
    ("repository", "add", "{newname}", "{url}", "{branch}"),
    ("repository", "delete", "{repo}"),
    ("show",), ("help",), ("end",), ("exit",),
    ("server", "{server}"), ("description", "{description}"),
)
GRAMMAR_SERVER = (("show",), ("help",), ("description", "{description}"), ("end",), ("exit",))


def _options(token: str, chosen: list[str], repositories: list[str]) -> list[str]:
    from .git_sync import list_cached_files
    if not token.startswith("{"):
        return [token]
    if token == "{repo}":
        return repositories
    if token == "{workspace}":
        from .workspace import list_files
        return list_files()
    if token in ("{inventory}", "{playbook}", "{allfiles}", "{file}"):
        if not chosen:
            return []
        # Repo is the last known positional token preceding dynamic filenames.
        repo = next((piece for piece in reversed(chosen)
                     if piece in repositories), None)
        if not repo:
            return []
        try:
            inventory = list_cached_files(repo, "inventory")
            playbooks = list_cached_files(repo, "playbooks")
        except (HTTPException, OSError):
            return []
        if token == "{inventory}":
            return inventory
        if token == "{playbook}":
            return playbooks
        return sorted(set(inventory + playbooks))
    if token == "{group}":
        if len(chosen) < 4:
            return []
        from .inventory_preview import preview_inventory
        try:
            preview = preview_inventory(chosen[2], chosen[3])
            return [item["name"] for item in preview["groups"] if item["host_count"] > 0]
        except (HTTPException, OSError):
            return []
    if token == "{sha}":
        repo = next((piece for piece in reversed(chosen) if piece in repositories), None)
        if not repo:
            return []
        from .git_changes import git_status
        try:
            return [git_status(repo)["head"]]
        except (HTTPException, OSError):
            return []
    # A free-form name, URL, commit message or description must be typed.
    return []


def _candidate_suffix(
    grammar: tuple[str, ...], fixed: list[str], partial: str, repos: list[str]
) -> list[str]:
    if len(fixed) >= len(grammar):
        return []
    chosen: list[str] = []
    for index, value in enumerate(fixed):
        pattern = grammar[index]
        if not pattern.startswith("{") and value.lower() != pattern:
            return []
        chosen.append(value)
    values = _options(grammar[len(fixed)], chosen, repos)
    return [v for v in values if v.lower().startswith(partial.lower())]


def context_complete(prefix: str, mode: str = "exec") -> tuple[list[str], str | None]:
    from .repositories import list_repositories

    raw = prefix.removeprefix("skr ")
    if len(raw) > 500 or "\n" in raw or "\r" in raw:
        return [], None
    # Do not tokenize with a shell: completion must never execute or expand input.
    trailing = bool(raw and raw[-1].isspace())
    parts = raw.split()
    fixed = parts if trailing else parts[:-1]
    partial = "" if trailing or not parts else parts[-1]
    if len(fixed) > 12:
        return [], None
    grammar = GRAMMAR_CONFIG if mode == "config" else (
        GRAMMAR_SERVER if mode == "config-server" else GRAMMAR_EXEC
    )
    repos = [entry["name"] for entry in list_repositories()]
    matched: list[str] = []
    for pattern in grammar:
        matched.extend(_candidate_suffix(pattern, fixed, partial, repos))

    if not matched:
        fallback = static_complete(raw, mode)
        # Free-form fields intentionally require manual input.
        return fallback, None

    # Each suggestion is a full line; the frontend can calculate the common
    # prefix and show alternatives without inserting literal placeholders.
    prefix_words = " ".join(fixed)
    choices = sorted(set(
        (prefix_words + " " if prefix_words else "") + token
        for token in matched
    ))
    if len(choices) == 1:
        # Follow unique known arguments until the next user choice or manual
        # input. This makes 'show hosts<Tab>' fill repo and inventory when unique.
        candidate = choices[0]
        for _ in range(8):
            complete_tokens = candidate.split()
            paths = [p for p in grammar if len(p) > len(complete_tokens) and
                     all((segment.startswith("{") or segment == value.lower())
                         for segment, value in zip(p, complete_tokens))]
            if not paths:
                break
            next_values: set[str] = set()
            for pattern in paths:
                next_values.update(_options(pattern[len(complete_tokens)], complete_tokens, repos))
            if len(next_values) != 1:
                break
            candidate += " " + next(iter(next_values))
        choices = [candidate]
    return choices[:100], None

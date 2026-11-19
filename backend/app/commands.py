"""Restricted SakuraOps command interpreter. Never executes a system shell."""
import shlex

VERSION = "0.2.0-dev"
ROOT = ("help", "show", "configure terminal", "edit", "nano", "files", "cat", "clear", "history", "exit", "end", "repository sync", "repository edit", "repository status", "repository commit", "repository push", "task plan")
SHOW = ("show version", "show status", "show servers", "show inventory", "show playbooks", "show commands", "show repositories", "show repository", "show hosts", "show groups")
CONFIG = ("help", "show", "end", "exit", "server", "description", "repository add", "repository delete")
CONFIG_SERVER = ("help", "show", "description", "end", "exit")

COMMANDS = {
    "help": "Pomoc w bieżącym trybie",
    "show": "Wyświetlanie informacji",
    "configure terminal": "Przejście do trybu konfiguracji",
    "edit <plik>": "Edytor tekstowy (pliki robocze)",
    "nano <plik>": "Alias edytora SakuraOps",
    "files": "Lista plików roboczych",
    "cat <plik>": "Odczyt pliku roboczego",
    "clear": "Wyczyść terminal",
    "exit": "Wyjście z bieżącego trybu",
    "show repositories": "Lista repozytoriów Git (konfiguracja)",
    "repository add <name> <url> [branch]": "Dodaj repozytorium w trybie konfiguracji",
    "repository delete <name>": "Usuń repozytorium w trybie konfiguracji",
    "repository sync <name>": "Ręcznie pobierz dane repozytorium (SSH, tylko odczyt)",
    "show repository <name> inventory|playbooks": "Lista plików z cache Git",
    "show repository <name> file <path>": "Pokaż zawartość pliku (do 64 KiB)",
    "show repository <name> diff <path>": "Pokaż lokalne różnice Git",
    "repository edit <name> <path>": "Edytuj lokalny plik inventory/playbook",
    "repository status <name>": "Zmiany plików oraz aktualny commit",
    "repository commit <name> <SHA> <message> <file...>": "Zatwierdź wybrane pliki (bez push)",
    "repository push <name> <SHA>": "Wyślij commit bez force (jawne potwierdzenie SHA)",
    "show hosts <repo> <inventory>": "Hosty ze statycznego inventory (bez wykonywania Ansible)",
    "show groups <repo> <inventory>": "Grupy i hosty ze statycznego inventory",
    "task plan <repo> <inventory> <playbook> <grupa>": "Podgląd celu zadania bez uruchamiania Ansible",
}


def _prompt(mode: str) -> str:
    return {
        "exec": "SakuraOps# ",
        "config": "SakuraOps(config)# ",
        "config-server": "SakuraOps(config-server)# ",
    }.get(mode, "SakuraOps# ")


def suggestions(mode: str) -> tuple[str, ...]:
    if mode == "config":
        return CONFIG
    if mode == "config-server":
        return CONFIG_SERVER
    return ROOT + SHOW


def complete(prefix: str, mode: str = "exec") -> list[str]:
    prefix = prefix.removeprefix("skr ").strip().lower()
    return sorted(x for x in suggestions(mode) if x.startswith(prefix))


def interpret(command: str, mode: str = "exec") -> dict:
    """Return structured actions; caller authorizes any IO action separately."""
    mode = mode if mode in ("exec", "config", "config-server") else "exec"
    prompt = _prompt(mode)
    raw = command.strip()
    if raw.startswith("skr "):
        raw = raw[4:]
    try:
        args = shlex.split(raw)
    except ValueError:
        return {"output": "Błąd składni.", "mode": mode, "prompt": prompt}
    if len(args) > 12:
        return {"output": "Zbyt wiele argumentów.", "mode": mode, "prompt": prompt}
    if not args:
        return {"output": "", "mode": mode, "prompt": prompt}
    lowered = [part.lower() for part in args]
    name = lowered[0]

    def result(message: str = "", new_mode: str | None = None, **extra):
        active = new_mode or mode
        return {"output": message, "mode": active, "prompt": _prompt(active), **extra}

    if name in ("help", "?"):
        return result("\n".join(COMMANDS if mode == "exec" else suggestions(mode)))
    if name == "repository" and mode == "exec" and len(args) == 3 and lowered[1] == "status":
        return result("", action="repository-status", name=args[2])
    if name == "repository" and mode == "exec" and len(args) >= 6 and lowered[1] == "commit":
        return result("", action="repository-commit", name=args[2],
                      expected_head=args[3], commit_message=args[4], files=args[5:])
    if name == "repository" and mode == "exec" and len(args) == 4 and lowered[1] == "push":
        return result("", action="repository-push", name=args[2], expected_head=args[3])
    if name == "repository" and mode == "exec" and len(args) == 4 and lowered[1] == "edit":
        return result("", action="repository-edit", name=args[2], filename=args[3])
    if name == "show" and len(args) == 5 and lowered[1] == "repository" and lowered[3] == "diff":
        return result("", action="repository-diff", name=args[2], filename=args[4])
    if name == "show" and len(args) == 5 and lowered[1] == "repository" and lowered[3] == "file":
        return result("", action="repository-read", name=args[2], filename=args[4])
    if name == "repository" and mode == "exec" and len(args) == 3 and lowered[1] == "sync":
        return result("", action="repository-sync", name=args[2])
    if name == "show" and len(args) == 4 and lowered[1] == "repository" and lowered[3] in ("inventory", "playbooks"):
        return result("", action="repository-files", name=args[2], kind=lowered[3])
    if name == "task" and len(args) == 6 and lowered[1] == "plan" and mode == "exec":
        return result("", action="task-plan", name=args[2], inventory=args[3],
                      playbook=args[4], group=args[5])
    if name == "show" and len(args) == 4 and lowered[1] in ("hosts", "groups"):
        return result("", action="inventory-preview", kind=lowered[1],
                      name=args[2], filename=args[3])
    if name == "show" and len(args) == 1:
        return result("Dostępne: version, status, servers, inventory, playbooks, commands, repositories, repository")
    if name == "show" and len(args) == 2:
        key = lowered[1]
        if key == "version":
            return result(f"SakuraOps {VERSION}")
        if key == "status":
            return result(f"SakuraOps: uruchomiony\nWersja: {VERSION}\nAnsible: niepodłączony\nKonsola: gotowa")
        if key == "repositories":
            return result("", action="repositories-list")
        if key == "commands":
            return result("\n".join(COMMANDS))
        if key == "servers":
            return result("Lista serwerów będzie dostępna po parsowaniu inventory.")
        if key in ("inventory", "playbooks"):
            return result("", action="repo-list", kind=key)
    if name in ("status", "version") and len(args) == 1 and mode == "exec":
        return interpret("show " + name, mode)
    if lowered == ["configure", "terminal"] and mode == "exec":
        return result("Tryb konfiguracji (bez zapisu zmian infrastruktury).", "config")
    if name == "repository" and mode == "config":
        if len(args) in (4, 5) and lowered[1] == "add":
            return result("", action="repository-add", name=args[2],
                          url=args[3], branch=args[4] if len(args) == 5 else "main")
        if len(args) == 3 and lowered[1] == "delete":
            return result("", action="repository-delete", name=args[2])
        return result("Składnia: repository add <nazwa> <url> [gałąź] lub repository delete <nazwa>")
    if name == "server" and mode == "config" and len(args) == 2:
        return result(f"Wybrano serwer: {args[1]} (tryb demonstracyjny).", "config-server")
    if name == "description" and mode.startswith("config"):
        return result("Zmiana opisu nie jest jeszcze zaimplementowana.")
    if name == "exit" and len(args) == 1:
        return result("", {"exec": "exec", "config": "exec", "config-server": "config"}[mode])
    if name == "end" and len(args) == 1:
        return result("", "exec")
    if name == "clear" and len(args) == 1:
        return result("", action="clear")
    if name == "history" and len(args) == 1:
        return result("Historia jest przechowywana w bieżącym oknie terminala.", action="history")
    if name == "files" and len(args) == 1 and mode == "exec":
        return result("", action="files")
    if name in ("edit", "nano", "cat") and len(args) == 2 and mode == "exec":
        return result("", action="read" if name == "cat" else "edit", filename=args[1])
    if raw.endswith("?"):
        return result("\n".join(complete(raw[:-1], mode)) or "Brak pasujących komend.")
    return result("Nieznane polecenie. Wpisz ? lub help.")


def execute(command: str) -> str:
    """Backward-compatible helper for existing tests."""
    return interpret(command)["output"]

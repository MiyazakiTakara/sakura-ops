import shlex

VERSION = "0.1.0-dev"

COMMANDS = {
    "help": "Wyświetla dostępne polecenia",
    "status": "Pokazuje stan SakuraOps",
    "version": "Pokazuje wersję systemu",
}


def execute(command: str) -> str:
    try:
        args = shlex.split(command)
    except ValueError:
        return "Błąd składni polecenia."

    if not args:
        return ""

    if args[0] == "skr":
        args = args[1:]

    if not args:
        return "Wpisz help, aby zobaczyć polecenia."

    name, *params = args

    if name == "help" and not params:
        return "\n".join(
            f"{cmd:<12} {description}"
            for cmd, description in COMMANDS.items()
        )

    if name == "version" and not params:
        return f"SakuraOps {VERSION}"

    if name == "status" and not params:
        return (
            "SakuraOps: uruchomiony\n"
            f"Wersja: {VERSION}\n"
            "Konsola: dostępna\n"
            "Ansible: jeszcze niepodłączony"
        )

    return f"Nieznane polecenie: {name}. Wpisz help."

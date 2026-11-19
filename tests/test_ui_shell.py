from pathlib import Path

from backend.app.commands import execute


def test_existing_console_commands():
    assert "SakuraOps" in execute("version")
    assert "Ansible: jeszcze niepodłączony" in execute("status")
    assert "help" in execute("skr help")


def test_console_rejects_unknown_shell_commands():
    assert "Nieznane polecenie" in execute("whoami")
    assert "Nieznane polecenie" in execute("bash -c 'id'")


def test_dashboard_contains_window_controls():
    dashboard = (
        Path(__file__).resolve().parents[1]
        / "backend/app/static/dashboard.html"
    ).read_text(encoding="utf-8")
    for fragment in (
        'id="open-terminal"',
        'id="minimize"',
        'id="maximize"',
        'id="close"',
        'id="command-form"',
        '"/api/console"',
        '"/api/me"',
        'sakuraops-csrf',
    ):
        assert fragment in dashboard

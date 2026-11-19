from pathlib import Path

from backend.app.commands import complete, execute, interpret
from backend.app.workspace import read_text, resolve_name, write_text
from fastapi import HTTPException
import pytest


def test_existing_console_commands():
    assert "SakuraOps" in execute("version")
    assert "Ansible: niepodłączony" in execute("status")
    assert "help" in execute("skr help")


def test_console_rejects_unknown_shell_commands():
    assert "Nieznane polecenie" in execute("whoami")
    assert "Nieznane polecenie" in execute("bash -c 'id'")


def test_cisco_modes():
    config = interpret("configure terminal")
    assert config["mode"] == "config"
    assert config["prompt"] == "SakuraOps(config)# "
    server = interpret("server glpi", "config")
    assert server["mode"] == "config-server"
    assert interpret("exit", "config-server")["mode"] == "config"
    assert interpret("end", "config-server")["mode"] == "exec"
    assert "show version" in complete("show v")


def test_editor_actions_do_not_execute_filesystem_calls():
    assert interpret("nano notes.md")["action"] == "edit"
    assert interpret("cat notes.md")["action"] == "read"
    assert interpret("files")["action"] == "files"


@pytest.mark.parametrize("name", ["../passwd", "/etc/passwd", ".secret", "evil.py", "..", "a/b.txt"])
def test_workspace_rejects_paths(name):
    with pytest.raises(HTTPException):
        resolve_name(name)


def test_workspace_roundtrip(tmp_path, monkeypatch):
    import backend.app.workspace as workspace
    monkeypatch.setattr(workspace, "WORKSPACE", tmp_path)
    write_text("test.yml", "hosts: all\n")
    assert read_text("test.yml") == "hosts: all\n"


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
        'id="xterm"',
        'id="editor-pane"',
        "sakuraops-csrf",
    ):
        assert fragment in dashboard

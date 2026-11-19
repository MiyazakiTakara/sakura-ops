from pathlib import Path

from backend.app.commands import interpret
from backend.app.inventory import discover


def test_read_only_inventory_command():
    assert interpret("show inventory")["action"] == "repo-list"
    assert interpret("show playbooks")["kind"] == "playbooks"
    assert "show inventory" in interpret("show commands")["output"] or "show" in interpret("show commands")["output"]


def test_discovery_requires_configured_git_checkout(monkeypatch):
    monkeypatch.delenv("SAKURAOPS_ANSIBLE_REPO_PATH", raising=False)
    heading, paths = discover("inventory")
    assert not paths
    assert "nie jest skonfigurowane" in heading


def test_discovery_lists_only_supported_files(tmp_path, monkeypatch):
    (tmp_path / ".git").mkdir()
    inventory = tmp_path / "inventories"
    inventory.mkdir()
    (inventory / "production.yml").write_text("all: {}")
    (inventory / "bad.sh").write_text("echo bad")
    (inventory / "link.yml").symlink_to(tmp_path / "bad.sh")
    playbooks = tmp_path / "playbooks"
    playbooks.mkdir()
    (playbooks / "audit.yaml").write_text("---")
    monkeypatch.setenv("SAKURAOPS_ANSIBLE_REPO_PATH", str(tmp_path))
    _, inventory_files = discover("inventory")
    _, playbook_files = discover("playbooks")
    assert inventory_files == ["inventories/production.yml"]
    assert playbook_files == ["playbooks/audit.yaml"]


def test_codemirror_editor_wired():
    root = Path(__file__).resolve().parents[1]
    js = (root / "frontend/src/terminal.js").read_text()
    html = (root / "backend/app/static/dashboard.html").read_text()
    assert 'from "codemirror"' in js
    assert "yaml()" in js
    assert "markdown()" in js
    assert 'id="editor-content"' in html

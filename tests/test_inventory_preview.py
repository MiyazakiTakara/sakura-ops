"""Passive Ansible inventory host/group parsing, without executing Ansible."""
import pytest
from fastapi import HTTPException

from backend.app import inventory_preview
from backend.app.commands import interpret


def test_static_ini_groups_children_and_hidden_vars(monkeypatch):
    text = """
[web]
web-01 ansible_host=10.0.0.1 ansible_password=SECRET
web-02

[db]
db-01 ansible_password=HIDDEN

[production:children]
web
db

[web:vars]
ansible_user=administrator
ansible_password=SECRET-2
"""
    monkeypatch.setattr(inventory_preview, "list_cached_files",
                        lambda repo, kind: ["inventory/hosts.ini"])
    monkeypatch.setattr(inventory_preview, "read_repository_file",
                        lambda repo, path: {"content": text})
    result = inventory_preview.preview_inventory("ansible", "inventory/hosts.ini")
    assert result["host_count"] == 3
    assert result["hosts"] == ["db-01", "web-01", "web-02"]
    prod = next(group for group in result["groups"] if group["name"] == "production")
    assert prod["hosts"] == ["db-01", "web-01", "web-02"]
    assert prod["children"] == ["db", "web"]
    assert "SECRET" not in str(result)
    assert "10.0.0.1" not in str(result)


def test_static_yaml_hosts_children_without_variable_values(monkeypatch):
    content = """
all:
  vars:
    ansible_password: dont-expose-me
  children:
    servers:
      hosts:
        app-1:
          ansible_host: 192.0.2.10
          secret: hidden
        app-2: {}
      children:
        db:
          hosts:
            postgres-1: {}
"""
    monkeypatch.setattr(inventory_preview, "list_cached_files",
                        lambda repo, kind: ["inventories/production.yaml"])
    monkeypatch.setattr(inventory_preview, "read_repository_file",
                        lambda repo, path: {"content": content})
    data = inventory_preview.preview_inventory("ansible", "inventories/production.yaml")
    assert data["host_count"] == 3
    assert data["groups"][0]["name"] == "all"
    assert data["groups"][0]["host_count"] == 3
    assert "dont-expose-me" not in str(data)
    assert "hidden" not in str(data)
    assert "192.0.2.10" not in str(data)


@pytest.mark.parametrize("content", [
    "[alpha:children]\nbeta\n[beta:children]\nalpha\n",
    "[web]\n../relative/path\n",
    "[web]\nhost-1\n[web:children]\nweb\n",
])
def test_ini_rejects_unsafe_or_cyclic_inventory(monkeypatch, content):
    monkeypatch.setattr(inventory_preview, "list_cached_files",
                        lambda repo, kind: ["inventory/hosts.ini"])
    monkeypatch.setattr(inventory_preview, "read_repository_file",
                        lambda repo, path: {"content": content})
    with pytest.raises(HTTPException):
        inventory_preview.preview_inventory("ansible", "inventory/hosts.ini")


def test_yaml_refuses_python_constructor_and_recursive_alias(monkeypatch):
    monkeypatch.setattr(inventory_preview, "list_cached_files",
                        lambda repo, kind: ["inventory/main.yml"])
    for content in [
        "all: !!python/object/apply:os.system ['echo unsafe']",
        "all: &a {children: {cycle: *a}}",
    ]:
        monkeypatch.setattr(inventory_preview, "read_repository_file",
                            lambda repo, path, text=content: {"content": text})
        with pytest.raises(HTTPException):
            inventory_preview.preview_inventory("ansible", "inventory/main.yml")


def test_only_git_tracked_inventory_and_no_secret_details(monkeypatch):
    monkeypatch.setattr(inventory_preview, "list_cached_files",
                        lambda repo, kind: ["inventory/hosts.ini"])
    with pytest.raises(HTTPException) as error:
        inventory_preview.preview_inventory("ansible", "../../.env")
    assert error.value.status_code == 404


def test_cli_and_page_expose_same_backend():
    assert interpret("show hosts ansible inventory/hosts.ini")["action"] == "inventory-preview"
    result = interpret("show groups ansible inventory/hosts.ini")
    assert result["kind"] == "groups"
    from pathlib import Path
    base = Path(__file__).resolve().parents[1]
    page = (base / "backend/app/static/dashboard.html").read_text()
    client = (base / "frontend/src/terminal.js").read_text()
    server = (base / "backend/app/main.py").read_text()
    assert 'id="nav-servers"' in page
    assert 'id="servers-groups"' in page
    assert 'inventory-preview?path=' in client
    assert '@app.get("/api/repositories/{name}/inventory-preview")' in server


def test_task_plan_targets_only_selected_static_group(monkeypatch):
    from backend.app import task_plans
    content = "[prod]\napp-1\napp-2\n[staging]\ntest-1\n"
    monkeypatch.setattr(inventory_preview, "list_cached_files",
                        lambda repo, kind: ["inventory/hosts.ini"])
    monkeypatch.setattr(inventory_preview, "read_repository_file",
                        lambda repo, path: {"content": content})
    monkeypatch.setattr(task_plans, "list_cached_files",
                        lambda repo, kind: ["playbooks/audit.yml"])
    monkeypatch.setattr(task_plans, "read_repository_file",
                        lambda repo, path: {"content": "- hosts: all"})
    plan = task_plans.task_plan("ansible", "inventory/hosts.ini", "playbooks/audit.yml", "prod")
    assert plan["hosts"] == ["app-1", "app-2"]
    assert plan["state"] == "preview_only"
    assert plan["executable"] is False
    with pytest.raises(HTTPException):
        task_plans.task_plan("ansible", "inventory/hosts.ini", "playbooks/other.yml", "prod")
    with pytest.raises(HTTPException):
        task_plans.task_plan("ansible", "inventory/hosts.ini", "playbooks/audit.yml", "unknown")
    with pytest.raises(HTTPException):
        task_plans.task_plan("ansible", "inventory/hosts.ini", "playbooks/audit.yml", "prod;echo")


def test_task_plan_cli_and_gui_wiring():
    parsed = interpret("task plan ansible inventory/hosts.ini playbooks/audit.yml prod")
    assert parsed["action"] == "task-plan"
    assert parsed["group"] == "prod"
    from pathlib import Path
    root = Path(__file__).resolve().parents[1]
    page = (root / "backend/app/static/dashboard.html").read_text()
    js = (root / "frontend/src/terminal.js").read_text()
    backend = (root / "backend/app/main.py").read_text()
    for item in ("nav-tasks", "task-repository", "task-playbook", "task-group", "task-plan-hosts"):
        assert 'id="' + item + '"' in page
    assert 'request("/api/tasks/plan"' in js
    assert '@app.post("/api/tasks/plan")' in backend
    assert 'require_console_session(request, edit=True)' in backend

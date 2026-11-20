"""Preflight security regression. No SSH, Git network or Ansible processes."""
import json
import pytest
from fastapi import HTTPException
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool
from backend.app import preflight
from backend.app.models import PreflightReview

SHA = "a" * 40
INV = "[lab]\nsemaphore\n"
PLAY = ("- name: Diagnostic\n  hosts: all\n  gather_facts: false\n  tasks:\n"
        "    - name: Connectivity\n      ansible.builtin.ping:\n")


@pytest.fixture
def prepared(monkeypatch):
    engine = create_engine("sqlite://", connect_args={"check_same_thread": False},
                           poolclass=StaticPool)
    PreflightReview.__table__.create(engine)
    monkeypatch.setattr(preflight, "SessionLocal", sessionmaker(bind=engine))
    monkeypatch.setenv("SAKURAOPS_PREFLIGHT_HOSTS", "semaphore")
    source = {"sha": SHA, "inv": INV, "play": PLAY}
    def snapshot(repo, inventory, playbook):
        return source["sha"], {"content": source["inv"], "revision": preflight.hashlib.sha256(source["inv"].encode()).hexdigest()}, {
            "content": source["play"], "revision": preflight.hashlib.sha256(source["play"].encode()).hexdigest()
        }
    monkeypatch.setattr(preflight, "_snapshot", snapshot)
    monkeypatch.setattr(preflight, "preview_inventory", lambda repo, path: {
        "groups": [{"name": "lab", "hosts": ["semaphore"]}]
    })
    yield source
    engine.dispose()


def create():
    return preflight.create("ansible", "inventory/hosts.ini", "playbooks/diagnostic.yml", "lab", "diagnostic", "admin")


def test_report_fingerprint_approval_and_history(prepared):
    report = create()
    assert report["state"] == "reviewable"
    assert report["report"]["git_sha"] == SHA
    assert report["report"]["modules"] == ["ansible.builtin.ping"]
    assert report["report"]["executable"] is False
    assert preflight.get(report["id"])["fingerprint"] == report["fingerprint"]
    approved = preflight.approve(report["id"], report["fingerprint"], "admin")
    assert approved["state"] == "approved"
    assert approved["approved_by"] == "admin"
    assert preflight.recent()[0]["id"] == report["id"]
    with pytest.raises(HTTPException):
        preflight.approve(report["id"], report["fingerprint"], "admin")


def test_stale_commit_or_changed_file_blocks_approval(prepared):
    report = create()
    prepared["sha"] = "b" * 40
    with pytest.raises(HTTPException) as exc:
        preflight.approve(report["id"], report["fingerprint"], "admin")
    assert exc.value.status_code == 409
    prepared["sha"] = SHA
    prepared["play"] += "    - name: Extra\n      ansible.builtin.ping:\n"
    with pytest.raises(HTTPException):
        preflight.approve(report["id"], report["fingerprint"], "admin")


@pytest.mark.parametrize("content", [
    "- hosts: all\n  gather_facts: false\n  tasks:\n    - ansible.builtin.shell: whoami\n",
    "- hosts: all\n  gather_facts: false\n  become: true\n  tasks:\n    - ansible.builtin.ping:\n",
    "- hosts: all\n  gather_facts: false\n  tasks:\n    - ansible.builtin.debug: {msg: '{{ secret }}'}\n",
    "- hosts: all\n  gather_facts: false\n  roles: [admin]\n  tasks:\n    - ansible.builtin.ping:\n",
    "- hosts: all\n  gather_facts: true\n  tasks:\n    - ansible.builtin.ping:\n",
    "- hosts: all\n  gather_facts: false\n  tasks:\n    - ansible.builtin.ping: {data: x}\n",
    "- hosts: &a all\n  gather_facts: false\n  tasks:\n    - ansible.builtin.ping:\n",
    "- hosts: all\n  gather_facts: false\n  tasks:\n    - ansible.builtin.debug: {msg: !!python/object/apply:os.system ['id']}\n",
])
def test_unsafe_playbook_is_blocked(prepared, content):
    prepared["play"] = content
    item = create()
    assert item["state"] == "blocked"
    assert item["report"]["blockers"]
    with pytest.raises(HTTPException):
        preflight.approve(item["id"], item["fingerprint"], "admin")


@pytest.mark.parametrize("inventory", [
    "[lab]\nsemaphore ansible_user=root\n",
    "[lab:vars]\nansible_password=secret\n[lab]\nsemaphore\n",
    "[lab]\nsemaphore ansible_host=192.0.2.10\n",
])
def test_ini_variables_are_not_approved(prepared, inventory):
    prepared["inv"] = inventory
    assert create()["state"] == "blocked"


def test_allowlist_and_fingerprint_fail_closed(prepared, monkeypatch):
    item = create()
    with pytest.raises(HTTPException):
        preflight.approve(item["id"], "0" * 64, "admin")
    monkeypatch.setenv("SAKURAOPS_PREFLIGHT_HOSTS", "")
    with pytest.raises(HTTPException):
        preflight.approve(item["id"], item["fingerprint"], "admin")
    assert create()["state"] == "blocked"


def test_no_dynamic_inventory_or_unknown_mode(prepared):
    prepared["inv"] = "[lab]\nsemaphore ansible_connection=local\n"
    assert create()["state"] == "blocked"
    with pytest.raises(HTTPException):
        preflight.create("ansible", "inventory/hosts.ini", "playbooks/diagnostic.yml",
                         "lab", "apply", "admin")


def test_git_safety_checks_before_reading(monkeypatch):
    monkeypatch.setattr(preflight, "list_cached_files",
                        lambda repo, kind: ["inventory/hosts.ini"] if kind == "inventory"
                        else ["playbooks/diagnostic.yml"])
    monkeypatch.setattr(preflight, "_location", lambda repo: "/tmp/fake-preflight")
    monkeypatch.setattr(preflight, "synchronization_status", lambda repo: {"commit_sha": SHA})
    monkeypatch.setattr(preflight, "_git", lambda args: "b" * 40 if "rev-parse" in args else "")
    monkeypatch.setattr(preflight, "read_repository_file",
                        lambda *args: pytest.fail("Must reject SHA mismatch before reading files"))
    with pytest.raises(HTTPException) as exc:
        preflight._snapshot("ansible", "inventory/hosts.ini", "playbooks/diagnostic.yml")
    assert exc.value.status_code == 409


def test_api_ui_wiring():
    from pathlib import Path
    root = Path(__file__).resolve().parents[1]
    api = (root / "backend/app/main.py").read_text()
    page = (root / "backend/app/static/dashboard.html").read_text()
    client = (root / "frontend/src/terminal.js").read_text()
    assert '@app.post("/api/preflight/reviews"' in api
    assert '@app.post("/api/preflight/reviews/{review_id}/approve")' in api
    assert 'require_console_session(request, edit=True)' in api
    for name in ("preflight-run", "preflight-approve", "preflight-details"):
        assert 'id="' + name + '"' in page
    assert '"/api/preflight/reviews"' in client

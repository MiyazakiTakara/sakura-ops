"""Dynamic Tab completion uses the same read-only repositories and inventory as GUI."""
import pytest

from backend.app import console_completion


@pytest.fixture
def completion_data(monkeypatch):
    from backend.app import git_sync, inventory_preview, git_changes, workspace

    monkeypatch.setattr("backend.app.repositories.list_repositories", lambda: [
        {"name": "ansible"}, {"name": "playground"},
    ])
    monkeypatch.setattr(git_sync, "list_cached_files",
                        lambda repo, kind: (
                            ["inventory/hosts.ini"] if kind == "inventory"
                            else ["playbooks/audit.yml", "playbooks/update.yml"]
                        ) if repo == "ansible" else (
                            ["inventory/test.ini"] if kind == "inventory"
                            else ["playbooks/check.yml"]
                        ))
    monkeypatch.setattr(inventory_preview, "preview_inventory",
                        lambda repo, path: {
                            "groups": [
                                {"name": "linux", "host_count": 2},
                                {"name": "production", "host_count": 1},
                                {"name": "empty", "host_count": 0},
                            ]
                        })
    monkeypatch.setattr(git_changes, "git_status",
                        lambda repo: {"head": "a" * 40})
    monkeypatch.setattr(workspace, "list_files", lambda: ["notes.md", "inventory.ini"])


def test_tab_offers_repository_names(completion_data):
    matches, hint = console_completion.context_complete("show hosts ")
    assert matches == ["show hosts ansible", "show hosts playground"]
    assert hint is None


def test_tab_fills_full_command_with_unique_inventory(completion_data):
    matches, hint = console_completion.context_complete("show hosts ans")
    assert matches == ["show hosts ansible inventory/hosts.ini"]
    matches, _ = console_completion.context_complete("show groups ansible ")
    assert matches == ["show groups ansible inventory/hosts.ini"]


def test_tab_completion_task_plan_nested_paths_and_groups(completion_data):
    matches, _ = console_completion.context_complete("task plan ansible ")
    assert matches == ["task plan ansible inventory/hosts.ini"]
    matches, _ = console_completion.context_complete(
        "task plan ansible inventory/hosts.ini "
    )
    assert matches == [
        "task plan ansible inventory/hosts.ini playbooks/audit.yml",
        "task plan ansible inventory/hosts.ini playbooks/update.yml",
    ]
    matches, _ = console_completion.context_complete(
        "task plan ansible inventory/hosts.ini playbooks/audit.yml "
    )
    assert matches == [
        "task plan ansible inventory/hosts.ini playbooks/audit.yml linux",
        "task plan ansible inventory/hosts.ini playbooks/audit.yml production",
    ]


def test_repository_and_workspace_commands(completion_data):
    matches, _ = console_completion.context_complete("repository sync ans")
    assert matches == ["repository sync ansible"]
    matches, _ = console_completion.context_complete("repository push ansible ")
    assert matches == ["repository push ansible " + "a" * 40]
    matches, _ = console_completion.context_complete("show repository ansible file inventory/")
    assert matches == ["show repository ansible file inventory/hosts.ini"]
    matches, _ = console_completion.context_complete("edit notes")
    assert matches == ["edit notes.md"]
    matches, _ = console_completion.context_complete("repository delete ans", "config")
    assert matches == ["repository delete ansible"]


def test_freeform_values_are_not_fabricated(completion_data):
    matches, _ = console_completion.context_complete("repository add", "config")
    assert matches == ["repository add"]
    matches, hint = console_completion.context_complete("repository add ", "config")
    assert matches == []
    assert "nazwę" in hint
    matches, hint = console_completion.context_complete(
        "repository commit ansible " + "a" * 40 + " "
    )
    assert matches == []
    assert "opis commita" in hint


def test_partial_and_static_commands(completion_data):
    matches, _ = console_completion.context_complete("repo")
    assert matches == ["repository"]
    matches, _ = console_completion.context_complete("show h")
    assert "show hosts" in matches
    matches, _ = console_completion.context_complete("show repositories")
    assert matches == ["show repositories"]
    assert console_completion.context_complete("show hosts\nrm -rf /")[0] == []


def test_aliases_and_config_mode_show_commands(completion_data):
    assert console_completion.context_complete("sta")[0] == ["status"]
    assert console_completion.context_complete("ver")[0] == ["version"]
    assert console_completion.context_complete("show hosts ans", "config")[0] == [
        "show hosts ansible inventory/hosts.ini"
    ]

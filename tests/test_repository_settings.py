import pytest
from fastapi import HTTPException
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from backend.app import repositories
from backend.app.commands import complete, interpret
from backend.app.models import GitRepository, GitSyncState


@pytest.fixture()
def repository_database(monkeypatch):
    engine = create_engine(
        "sqlite://", connect_args={"check_same_thread": False}, poolclass=StaticPool
    )
    GitRepository.__table__.create(bind=engine)
    GitSyncState.__table__.create(bind=engine)
    monkeypatch.setattr(repositories, "SessionLocal", sessionmaker(bind=engine))
    yield
    engine.dispose()


def test_cli_actions_restrict_mutation_to_config_mode():
    assert "Nieznane polecenie" in interpret(
        "repository add ansible https://git.example.org/a/r.git"
    )["output"]
    conf = interpret("repository add ansible https://git.example.org/a/r.git", "config")
    assert conf["action"] == "repository-add"
    assert conf["name"] == "ansible"
    assert conf["branch"] == "main"
    assert interpret("repository delete ansible", "config")["action"] == "repository-delete"
    assert interpret("show repositories")["action"] == "repositories-list"
    assert "show repositories" in complete("show repo")
    assert "repository add" in complete("repository a", "config")


def test_cli_gui_share_database_service(repository_database):
    saved = repositories.add_repository(
        "ansible", "https://git.example.org/it/ansible.git", "main"
    )
    assert saved["status"] == "configured"
    assert saved["sync_enabled"] is False
    assert repositories.list_repositories() == [saved]
    with pytest.raises(HTTPException) as duplicate:
        repositories.add_repository("ansible", saved["url"])
    assert duplicate.value.status_code == 409
    repositories.delete_repository("ansible")
    assert repositories.list_repositories() == []


@pytest.mark.parametrize(
    "url",
    [
        "http://git.example.org/team/ansible",
        "https://user:secret@git.example.org/team/ansible",
        "https://git.example.org/team/ansible?token=abc",
        "https://git.example.org/",
        "file:///etc/passwd",
        "ssh://root@git.example.org/team/ansible",
        "https://git.example.org:xyz/team/ansible",
    ],
)
def test_rejects_bad_repository_urls(url):
    with pytest.raises(HTTPException):
        repositories.validate("ansible", url, "main")


def test_ssh_configuration_is_metadata_only(repository_database):
    repo = repositories.add_repository(
        "ansible", "ssh://git@git.example.org/team/ansible.git", "release"
    )
    assert repo["sync_enabled"] is False
    assert repo["url"].startswith("ssh://")


def test_gui_fields_and_shared_endpoint_are_present():
    from pathlib import Path

    root = Path(__file__).resolve().parents[1]
    html = (root / "backend/app/static/dashboard.html").read_text(encoding="utf-8")
    js = (root / "frontend/src/terminal.js").read_text(encoding="utf-8")
    api = (root / "backend/app/main.py").read_text(encoding="utf-8")
    assert 'id="repo-form"' in html
    assert 'id="repo-list"' in html
    assert 'id="nav-repositories"' in html
    assert '"/api/repositories"' in js
    assert '@app.post("/api/repositories"' in api
    assert "require_console_session(request, edit=True)" in api


def test_repository_panel_uses_sse_with_fallback():
    from pathlib import Path
    root = Path(__file__).resolve().parents[1]
    js = (root / "frontend/src/terminal.js").read_text(encoding="utf-8")
    api = (root / "backend/app/main.py").read_text(encoding="utf-8")
    assert 'new EventSource("/api/repositories/events")' in js
    assert 'addEventListener("repositories-changed", updateRepositoriesWhenVisible)' in js
    assert "FALLBACK_REFRESH_MS = 30000" in js
    assert "repoRefreshQueued = true" in js
    assert "window.location.reload" not in js
    assert '@app.get("/api/repositories/events")' in api
    assert 'media_type="text/event-stream"' in api
    assert "user[\"role\"] != \"admin\"" in api


def test_sse_fingerprint_changes_with_repository_configuration(repository_database):
    from backend.app.repositories import repository_fingerprint
    before = repository_fingerprint()
    repositories.add_repository("ansible", "https://git.example.org/a/repo.git")
    after = repository_fingerprint()
    assert before != after
    repositories.delete_repository("ansible")
    assert repository_fingerprint() == before


def test_normal_git_first_clone_then_fast_forward(repository_database, monkeypatch, tmp_path):
    from backend.app import git_sync
    monkeypatch.setattr(git_sync, "SessionLocal", repositories.SessionLocal)
    monkeypatch.setattr(git_sync, "WORKTREES", tmp_path)
    calls = []
    url = "https://git.example.org/team/ansible.git"

    def fake_git(args, timeout=90):
        calls.append(args)
        if "clone" in args:
            folder = tmp_path / args[-1] if not args[-1].startswith("/") else __import__("pathlib").Path(args[-1])
            folder.mkdir()
            (folder / ".git").mkdir()
        if "rev-parse" in args:
            return "a" * 40
        if "remote.origin.url" in args:
            return url
        if "branch" in args:
            return "main"
        if "ls-files" in args:
            return "\n".join(["inventory/production.yml", "playbooks/update.yaml"])
        return ""

    monkeypatch.setattr(git_sync, "_git", fake_git)
    repositories.add_repository("ansible", url)
    first = git_sync.synchronize("ansible")
    assert first["commit_sha"] == "a" * 40
    assert (tmp_path / "ansible" / ".git").is_dir()
    assert git_sync.list_cached_files("ansible", "inventory") == ["inventory/production.yml"]
    second = git_sync.synchronize("ansible")
    assert second["commit_sha"] == first["commit_sha"]
    assert sum(1 for cmd in calls if "clone" in cmd) == 1
    assert sum(1 for cmd in calls if "fetch" in cmd) == 1
    assert any("merge" in cmd and "--ff-only" in cmd for cmd in calls)
    assert not any("push" in cmd for cmd in calls)


def test_dirty_worktree_is_not_overwritten(repository_database, monkeypatch, tmp_path):
    from backend.app import git_sync
    monkeypatch.setattr(git_sync, "SessionLocal", repositories.SessionLocal)
    monkeypatch.setattr(git_sync, "WORKTREES", tmp_path)
    target = tmp_path / "ansible"
    (target / ".git").mkdir(parents=True)
    url = "https://git.example.org/team/ansible.git"
    repositories.add_repository("ansible", url)
    commands = []

    def fake_git(args, timeout=90):
        commands.append(args)
        if "remote.origin.url" in args:
            return url
        if "branch" in args:
            return "main"
        if "status" in args:
            return " M inventory/prod.yml"
        return ""

    monkeypatch.setattr(git_sync, "_git", fake_git)
    with pytest.raises(HTTPException) as error:
        git_sync.synchronize("ansible")
    assert error.value.status_code == 409
    assert not any("fetch" in cmd or "merge" in cmd for cmd in commands)


def test_git_sync_does_not_use_unapproved_hosts(monkeypatch):
    from backend.app.git_sync import _check_remote
    monkeypatch.setenv("SAKURAOPS_GIT_ALLOWED_HOSTS", "git.example.org")
    _check_remote("https://git.example.org/team/repo")
    _check_remote("ssh://git@git.example.org/team/repo")
    with pytest.raises(HTTPException):
        _check_remote("file:///etc/passwd")
    with pytest.raises(HTTPException):
        _check_remote("https://other.example.org/team/repo")


def test_sync_console_actions():
    assert interpret("repository sync ansible")["action"] == "repository-sync"
    result = interpret("show repository ansible inventory")
    assert result["action"] == "repository-files"
    assert result["kind"] == "inventory"


def test_git_file_read_write_and_revision_conflict(repository_database, monkeypatch, tmp_path):
    from backend.app import git_sync
    monkeypatch.setattr(git_sync, "SessionLocal", repositories.SessionLocal)
    monkeypatch.setattr(git_sync, "WORKTREES", tmp_path)
    repositories.add_repository("ansible", "https://git.example.org/team/r.git")
    root = tmp_path / "ansible"
    (root / ".git").mkdir(parents=True)
    (root / "inventory").mkdir()
    target = root / "inventory" / "hosts.ini"
    target.write_text("[all]\\nnode1\\n", encoding="utf-8")
    monkeypatch.setattr(git_sync, "list_cached_files",
                        lambda name, kind: ["inventory/hosts.ini"] if kind == "inventory" else [])

    original = git_sync.read_repository_file("ansible", "inventory/hosts.ini")
    assert "[all]" in original["content"]
    saved = git_sync.write_repository_file(
        "ansible", "inventory/hosts.ini", "[all]\nnode2\n", original["revision"]
    )
    assert saved["content"] == "[all]\nnode2\n"
    assert saved["revision"] != original["revision"]
    with pytest.raises(HTTPException) as conflict:
        git_sync.write_repository_file("ansible", "inventory/hosts.ini",
                                       "wrong", original["revision"])
    assert conflict.value.status_code == 409
    assert target.read_text() == "[all]\nnode2\n"


def test_git_file_rejects_escape_symlink_and_untracked(repository_database, monkeypatch, tmp_path):
    from backend.app import git_sync
    monkeypatch.setattr(git_sync, "SessionLocal", repositories.SessionLocal)
    monkeypatch.setattr(git_sync, "WORKTREES", tmp_path)
    repositories.add_repository("ansible", "https://git.example.org/team/r.git")
    root = tmp_path / "ansible"
    (root / ".git").mkdir(parents=True)
    (root / "inventory").mkdir()
    external = tmp_path / "outside.ini"
    external.write_text("secret", encoding="utf-8")
    (root / "inventory" / "linked.ini").symlink_to(external)
    monkeypatch.setattr(git_sync, "list_cached_files",
                        lambda name, kind: ["inventory/linked.ini"] if kind == "inventory" else [])
    for path in ("../outside.ini", "inventory/../../outside.ini", "/etc/passwd",
                 "playbooks/other.yaml", "inventory/unknown.ini", "inventory/linked.ini"):
        with pytest.raises(HTTPException):
            git_sync.read_repository_file("ansible", path)
    assert external.read_text() == "secret"


def test_git_file_rejects_oversize(repository_database, monkeypatch, tmp_path):
    from backend.app import git_sync
    monkeypatch.setattr(git_sync, "SessionLocal", repositories.SessionLocal)
    monkeypatch.setattr(git_sync, "WORKTREES", tmp_path)
    repositories.add_repository("ansible", "https://git.example.org/team/r.git")
    root = tmp_path / "ansible"
    (root / ".git").mkdir(parents=True)
    (root / "playbooks").mkdir()
    file = root / "playbooks" / "test.yml"
    file.write_bytes(b"x" * 65537)
    monkeypatch.setattr(git_sync, "list_cached_files",
                        lambda name, kind: ["playbooks/test.yml"] if kind == "playbooks" else [])
    with pytest.raises(HTTPException) as e:
        git_sync.read_repository_file("ansible", "playbooks/test.yml")
    assert e.value.status_code == 413


def test_git_cli_edit_and_read_actions():
    assert interpret("repository edit ansible inventory/hosts.ini")["action"] == "repository-edit"
    result = interpret("show repository ansible file playbooks/audit.yml")
    assert result["action"] == "repository-read"
    assert result["filename"] == "playbooks/audit.yml"


def test_git_file_pages_api_and_editor_present():
    from pathlib import Path
    base = Path(__file__).resolve().parents[1]
    html = (base / "backend/app/static/dashboard.html").read_text()
    js = (base / "frontend/src/terminal.js").read_text()
    api = (base / "backend/app/main.py").read_text()
    for element in ("nav-inventory", "nav-playbooks", "git-file-editor", "git-file-save"):
        assert f'id="{element}"' in html
    assert "new EditorView" in js and "gitFileRevision" in js
    assert '@app.put("/api/repositories/{name}/content")' in api
    assert "require_console_session(request, edit=True)" in api

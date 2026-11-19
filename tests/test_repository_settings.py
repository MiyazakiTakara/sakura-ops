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


def test_manual_sync_uses_pinned_ssh_and_no_checkout(repository_database, monkeypatch, tmp_path):
    from backend.app import git_sync
    monkeypatch.setattr(git_sync, "SessionLocal", repositories.SessionLocal)
    root = tmp_path / "mirrors"
    root.mkdir()
    monkeypatch.setattr(git_sync, "MIRRORS", root)
    monkeypatch.setenv("SAKURAOPS_GIT_ALLOWED_HOSTS", "git.example.org")
    calls = []

    def fake_settings(url):
        assert url.startswith("ssh://git@")
        return "git.example.org", ["ssh", "-F", "/dev/null"]

    def fake_git(args, ssh=None, credentials=None):
        calls.append((args, ssh))
        if "rev-parse" in args:
            return "a" * 40
        if "ls-tree" in args:
            return "\n".join(["inventory/prod.yml", "playbooks/audit.yaml", "README.md"])
        return ""

    monkeypatch.setattr(git_sync, "_settings", fake_settings)
    monkeypatch.setattr(git_sync, "_git", fake_git)
    repositories.add_repository("ansible", "ssh://git@git.example.org/team/ansible.git")
    result = git_sync.synchronize("ansible")
    assert result["commit_sha"] == "a" * 40
    assert git_sync.list_cached_files("ansible", "inventory") == ["inventory/prod.yml"]
    assert git_sync.list_cached_files("ansible", "playbooks") == ["playbooks/audit.yaml"]
    assert not any("checkout" in parts for parts, _ in calls)
    assert not any("push" in parts for parts, _ in calls)
    assert any("fetch" in parts and ssh is not None for parts, ssh in calls)
    rows = repositories.list_repositories()
    assert rows[0]["sync_status"] == "synced"


def test_ssh_sync_is_disabled_without_operator_approval(monkeypatch):
    from backend.app.git_sync import _settings
    monkeypatch.delenv("SAKURAOPS_GIT_ALLOWED_HOSTS", raising=False)
    with pytest.raises(HTTPException) as exc:
        _settings("ssh://git@git.example.org/team/ansible.git")
    assert exc.value.status_code == 403


def test_sync_console_actions():
    assert interpret("repository sync ansible")["action"] == "repository-sync"
    result = interpret("show repository ansible inventory")
    assert result["action"] == "repository-files"
    assert result["kind"] == "inventory"


def test_https_sync_without_token_uses_allowlisted_host(repository_database, monkeypatch, tmp_path):
    from backend.app import git_sync
    monkeypatch.setattr(git_sync, "SessionLocal", repositories.SessionLocal)
    mirrors = tmp_path / "mirrors"
    mirrors.mkdir()
    monkeypatch.setattr(git_sync, "MIRRORS", mirrors)
    monkeypatch.setenv("SAKURAOPS_GIT_ALLOWED_HOSTS", "git.example.org")
    calls = []

    def fake_git(args, ssh=None, credentials=None):
        calls.append((args, ssh, credentials))
        if "rev-parse" in args:
            return "b" * 40
        return ""

    monkeypatch.setattr(git_sync, "_git", fake_git)
    repositories.add_repository("httpsrepo", "https://git.example.org/team/ansible.git")
    result = git_sync.synchronize("httpsrepo")
    assert result["commit_sha"] == "b" * 40
    assert any("fetch" in args and ssh is None and not credentials
               for args, ssh, credentials in calls)


def test_https_token_requires_private_file(monkeypatch, tmp_path):
    from backend.app.git_sync import _https_credentials
    token_file = tmp_path / "token"
    token_file.write_text("secret")
    monkeypatch.setenv("SAKURAOPS_GIT_TOKEN_FILE_ANSIBLE", str(token_file))
    with pytest.raises(HTTPException):
        _https_credentials("ansible")
    token_file.chmod(0o600)
    credentials = _https_credentials("ansible")
    import base64
    assert base64.b64decode(credentials["GIT_CONFIG_VALUE_0"].split("Basic ")[1]) == b"oauth2:secret"
    assert "secret" not in str(credentials.keys())


def test_https_rejects_hosts_not_allowlisted(monkeypatch):
    from backend.app.git_sync import _allowed_host
    monkeypatch.setenv("SAKURAOPS_GIT_ALLOWED_HOSTS", "git.example.org")
    with pytest.raises(HTTPException) as exc:
        _allowed_host("https://unknown.example.org/team/ansible")
    assert exc.value.status_code == 403
    assert _allowed_host("https://git.example.org/team/ansible")[0] == "https"

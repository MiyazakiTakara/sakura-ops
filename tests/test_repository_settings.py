import pytest
from fastapi import HTTPException
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from backend.app import repositories
from backend.app.commands import complete, interpret
from backend.app.models import GitRepository


@pytest.fixture()
def repository_database(monkeypatch):
    engine = create_engine(
        "sqlite://", connect_args={"check_same_thread": False}, poolclass=StaticPool
    )
    GitRepository.__table__.create(bind=engine)
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

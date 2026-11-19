"""End-to-end Git change management on real local repositories (no network)."""
import os
import subprocess

import pytest
from fastapi import HTTPException
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from sqlalchemy.pool import StaticPool

from backend.app import git_changes, git_sync, repositories
from backend.app.commands import interpret
from backend.app.models import GitRepository, GitSyncState


def git(*args):
    result = subprocess.run(["git", *map(str, args)], check=True, capture_output=True,
                            text=True)
    return result.stdout.strip()


@pytest.fixture
def working_repo(monkeypatch, tmp_path):
    engine = create_engine("sqlite://", connect_args={"check_same_thread": False},
                           poolclass=StaticPool)
    GitRepository.__table__.create(bind=engine)
    GitSyncState.__table__.create(bind=engine)
    session = sessionmaker(bind=engine)
    monkeypatch.setattr(git_sync, "SessionLocal", session)
    monkeypatch.setattr(repositories, "SessionLocal", session)
    monkeypatch.setattr(git_sync, "WORKTREES", tmp_path / "worktrees")
    git_sync.WORKTREES.mkdir()
    monkeypatch.setattr(git_sync, "_check_remote", lambda url: None)

    bare = tmp_path / "remote.git"
    root = git_sync.WORKTREES / "ansible"
    git("init", "--bare", bare)
    git("init", "-b", "main", root)
    git("-C", root, "config", "user.name", "Sakura Test")
    git("-C", root, "config", "user.email", "test@example.invalid")
    for directory in ("inventory", "playbooks"):
        (root / directory).mkdir()
    (root / "inventory" / "hosts.ini").write_text("[all]\nnode1\n")
    (root / "playbooks" / "audit.yml").write_text("- hosts: all\n")
    git("-C", root, "add", "inventory/hosts.ini", "playbooks/audit.yml")
    git("-C", root, "commit", "-m", "baseline")
    git("-C", root, "remote", "add", "origin", str(bare))
    git("-C", root, "push", "-u", "origin", "main")
    with session() as db:
        db.add(GitRepository(name="ansible", url=str(bare), branch="main"))
        db.commit()
    original_git = git_sync._git

    def git_with_test_file_push(args, timeout=90):
        if "push" in args:
            # Local file transport is enabled only in this test adapter.
            env = dict(os.environ)
            env["GIT_ALLOW_PROTOCOL"] = "file:https:ssh"
            outcome = subprocess.run(
                ["git", "-c", "core.hooksPath=/dev/null", *args],
                env=env, capture_output=True, text=True, timeout=timeout,
            )
            if outcome.returncode:
                raise HTTPException(502, "Test push rejected")
            return outcome.stdout.strip()
        return original_git(args, timeout)

    monkeypatch.setattr(git_sync, "_git", git_with_test_file_push)
    yield root, bare
    engine.dispose()


def test_selective_commit_and_nonforce_push(working_repo):
    root, bare = working_repo
    old = git("-C", root, "rev-parse", "HEAD")
    (root / "inventory" / "hosts.ini").write_text("[all]\nnode2\n")
    (root / "playbooks" / "audit.yml").write_text("- hosts: localhost\n")
    status = git_changes.git_status("ansible")
    assert status["head"] == old
    assert status["ahead"] == 0
    assert status["can_push"] is False
    assert status["modified_files"] == ["inventory/hosts.ini", "playbooks/audit.yml"]
    committed = git_changes.commit_selected(
        "ansible", ["inventory/hosts.ini"], "Update inventory", old
    )
    assert committed["commit_sha"] != old
    assert git_changes.git_status("ansible")["ahead"] == 1
    assert git_changes.git_status("ansible")["can_push"] is True
    assert git("-C", root, "show", "--format=", "--name-only", "HEAD") == "inventory/hosts.ini"
    assert git_changes.git_status("ansible")["modified_files"] == ["playbooks/audit.yml"]
    assert git("-C", bare, "rev-parse", "refs/heads/main") == old

    with pytest.raises(HTTPException) as stale:
        git_changes.push_explicit("ansible", old)
    assert stale.value.status_code == 409
    pushed = git_changes.push_explicit("ansible", committed["commit_sha"])
    assert pushed["pushed_sha"] == committed["commit_sha"]
    assert git("-C", bare, "rev-parse", "refs/heads/main") == committed["commit_sha"]
    assert git_changes.git_status("ansible")["ahead"] == 0
    assert git_changes.git_status("ansible")["can_push"] is False


def test_push_denied_when_no_commits_ahead(working_repo):
    root, bare = working_repo
    sha = git("-C", root, "rev-parse", "HEAD")
    current = git("-C", bare, "rev-parse", "refs/heads/main")
    assert git_changes.git_status("ansible")["ahead"] == 0
    with pytest.raises(HTTPException) as error:
        git_changes.push_explicit("ansible", sha)
    assert error.value.status_code == 409
    assert git("-C", bare, "rev-parse", "refs/heads/main") == current


def test_push_denied_without_tracking_ref(working_repo):
    root, _ = working_repo
    sha = git("-C", root, "rev-parse", "HEAD")
    git("-C", root, "update-ref", "-d", "refs/remotes/origin/main")
    status = git_changes.git_status("ansible")
    assert status["ahead"] is None
    assert status["can_push"] is False
    with pytest.raises(HTTPException) as error:
        git_changes.push_explicit("ansible", sha)
    assert error.value.status_code == 409


def test_commit_rejects_unknown_file_stale_sha_and_empty_message(working_repo):
    root, bare = working_repo
    head = git("-C", root, "rev-parse", "HEAD")
    (root / "inventory" / "hosts.ini").write_text("[all]\nnew\n")
    for files, message, expected in [
        (["inventory/../.git/config"], "change", head),
        (["playbooks/audit.yml"], "change", head),
        (["inventory/hosts.ini"], "", head),
        (["inventory/hosts.ini"], "change", "0" * 40),
    ]:
        with pytest.raises(HTTPException):
            git_changes.commit_selected("ansible", files, message, expected)
    assert git("-C", root, "rev-parse", "HEAD") == head


def test_nonfastforward_push_is_rejected(working_repo, tmp_path):
    root, bare = working_repo
    other = tmp_path / "other"
    git("clone", "-b", "main", bare, other)
    git("-C", other, "config", "user.name", "Other")
    git("-C", other, "config", "user.email", "other@example.invalid")
    (other / "playbooks" / "audit.yml").write_text("- hosts: changed\n")
    git("-C", other, "add", "playbooks/audit.yml")
    git("-C", other, "commit", "-m", "remote update")
    git("-C", other, "push", "origin", "HEAD:main")
    remote_sha = git("-C", bare, "rev-parse", "refs/heads/main")

    (root / "inventory" / "hosts.ini").write_text("[all]\nlocal\n")
    head = git("-C", root, "rev-parse", "HEAD")
    local_sha = git_changes.commit_selected(
        "ansible", ["inventory/hosts.ini"], "local update", head
    )["commit_sha"]
    with pytest.raises(HTTPException):
        git_changes.push_explicit("ansible", local_sha)
    assert git("-C", bare, "rev-parse", "refs/heads/main") == remote_sha


def test_console_git_actions():
    assert interpret("repository status ansible")["action"] == "repository-status"
    commit = interpret('repository commit ansible ' + "a" * 40
                       + ' "Update hosts" inventory/hosts.ini')
    assert commit["action"] == "repository-commit"
    assert commit["commit_message"] == "Update hosts"
    assert commit["files"] == ["inventory/hosts.ini"]
    assert interpret("repository push ansible " + "a" * 40)["action"] == "repository-push"
    assert "Nieznane polecenie" in interpret("repository push ansible")["output"]

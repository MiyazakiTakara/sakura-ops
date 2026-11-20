from sqlalchemy import Boolean, Integer, String
from sqlalchemy.orm import Mapped, mapped_column

from .database import Base


class User(Base):
    __tablename__ = "users"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    username: Mapped[str] = mapped_column(String(100), unique=True)
    password_hash: Mapped[str] = mapped_column(String(255))
    role: Mapped[str] = mapped_column(String(30), default="viewer")
    active: Mapped[bool] = mapped_column(Boolean, default=True)


class GitRepository(Base):
    __tablename__ = "git_repositories"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    name: Mapped[str] = mapped_column(String(40), unique=True, nullable=False)
    url: Mapped[str] = mapped_column(String(500), nullable=False)
    branch: Mapped[str] = mapped_column(String(100), nullable=False, default="main")


class GitSyncState(Base):
    __tablename__ = "git_sync_states"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    repository_name: Mapped[str] = mapped_column(String(40), unique=True, nullable=False)
    commit_sha: Mapped[str] = mapped_column(String(40), nullable=False)
    synced_at: Mapped[str] = mapped_column(String(40), nullable=False)


class LocalTestJob(Base):
    """A job has no inventory, playbook path or remote-target fields by design."""
    __tablename__ = "local_test_jobs"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    status: Mapped[str] = mapped_column(String(20), nullable=False, default="queued")
    created_at: Mapped[str] = mapped_column(String(40), nullable=False)
    started_at: Mapped[str | None] = mapped_column(String(40), nullable=True)
    finished_at: Mapped[str | None] = mapped_column(String(40), nullable=True)
    exit_code: Mapped[int | None] = mapped_column(Integer, nullable=True)
    log: Mapped[str] = mapped_column(String(65536), nullable=False, default="")
    cancel_requested: Mapped[bool] = mapped_column(Boolean, nullable=False, default=False)


class SSHDiagnosticJob(Base):
    """Explicit SSH connectivity probe for an administrator allowlisted IP."""
    __tablename__ = "ssh_diagnostic_jobs"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    alias: Mapped[str] = mapped_column(String(40), nullable=False)
    address: Mapped[str] = mapped_column(String(45), nullable=False)
    status: Mapped[str] = mapped_column(String(20), nullable=False, default="queued")
    created_at: Mapped[str] = mapped_column(String(40), nullable=False)
    started_at: Mapped[str | None] = mapped_column(String(40), nullable=True)
    finished_at: Mapped[str | None] = mapped_column(String(40), nullable=True)
    exit_code: Mapped[int | None] = mapped_column(Integer, nullable=True)
    log: Mapped[str] = mapped_column(String(16384), nullable=False, default="")
    cancel_requested: Mapped[bool] = mapped_column(Boolean, nullable=False, default=False)


class AnsiblePingJob(Base):
    """One-host, builtin Ansible ping diagnostic; never accepts a playbook."""
    __tablename__ = "ansible_ping_jobs"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    alias: Mapped[str] = mapped_column(String(40), nullable=False)
    address: Mapped[str] = mapped_column(String(45), nullable=False)
    status: Mapped[str] = mapped_column(String(20), nullable=False, default="queued")
    created_at: Mapped[str] = mapped_column(String(40), nullable=False)
    started_at: Mapped[str | None] = mapped_column(String(40), nullable=True)
    finished_at: Mapped[str | None] = mapped_column(String(40), nullable=True)
    exit_code: Mapped[int | None] = mapped_column(Integer, nullable=True)
    log: Mapped[str] = mapped_column(String(16384), nullable=False, default="")
    cancel_requested: Mapped[bool] = mapped_column(Boolean, nullable=False, default=False)


class PreflightReview(Base):
    """An immutable non-executing assessment and optional admin acknowledgement."""
    __tablename__ = "preflight_reviews"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    repository: Mapped[str] = mapped_column(String(40), nullable=False)
    inventory: Mapped[str] = mapped_column(String(500), nullable=False)
    playbook: Mapped[str] = mapped_column(String(500), nullable=False)
    target_group: Mapped[str] = mapped_column(String(255), nullable=False)
    mode: Mapped[str] = mapped_column(String(20), nullable=False)
    git_sha: Mapped[str] = mapped_column(String(40), nullable=False)
    inventory_digest: Mapped[str] = mapped_column(String(64), nullable=False)
    playbook_digest: Mapped[str] = mapped_column(String(64), nullable=False)
    report_json: Mapped[str] = mapped_column(String(32000), nullable=False)
    fingerprint: Mapped[str] = mapped_column(String(64), nullable=False)
    state: Mapped[str] = mapped_column(String(20), nullable=False)
    created_by: Mapped[str] = mapped_column(String(100), nullable=False)
    created_at: Mapped[str] = mapped_column(String(40), nullable=False)
    approved_by: Mapped[str | None] = mapped_column(String(100), nullable=True)
    approved_at: Mapped[str | None] = mapped_column(String(40), nullable=True)

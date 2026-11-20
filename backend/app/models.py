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

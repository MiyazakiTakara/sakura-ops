from pathlib import Path

from sqlalchemy import create_engine
from sqlalchemy.orm import DeclarativeBase, sessionmaker

DATA_DIR = Path(__file__).resolve().parents[2] / "data"
DATA_DIR.mkdir(parents=True, exist_ok=True)

engine = create_engine(
    f"sqlite:///{DATA_DIR / 'sakuraops.db'}"
)

SessionLocal = sessionmaker(bind=engine)


class Base(DeclarativeBase):
    pass

import getpass
import sys

from argon2 import PasswordHasher
from sqlalchemy import select

from app.database import Base, SessionLocal, engine
from app.models import User

Base.metadata.create_all(engine)

with SessionLocal() as db:
    if db.scalar(select(User.id).limit(1)) is not None:
        sys.exit("Konto już istnieje. Inicjalizacja administratora zablokowana.")

    username = input("Login administratora: ").strip()
    if not username:
        sys.exit("Login nie może być pusty.")

    password = getpass.getpass("Hasło: ")
    confirm = getpass.getpass("Powtórz hasło: ")

    if password != confirm or len(password) < 12:
        sys.exit("Hasła muszą być identyczne i mieć minimum 12 znaków.")

    user = User(
        username=username,
        password_hash=PasswordHasher().hash(password),
        role="admin",
    )

    db.add(user)
    db.commit()

print("Administrator SakuraOps został utworzony.")

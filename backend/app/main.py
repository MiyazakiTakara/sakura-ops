import hmac
import html
import os
import secrets
from pathlib import Path

from argon2 import PasswordHasher
from argon2.exceptions import VerificationError
from fastapi import FastAPI, Form, HTTPException, Request
from fastapi.responses import HTMLResponse, RedirectResponse
from sqlalchemy import select
from starlette.middleware.sessions import SessionMiddleware

from .database import SessionLocal
from .models import User

secret_file = Path(__file__).resolve().parents[2] / ".secrets/session_key"
secret = secret_file.read_text().strip()

app = FastAPI(title="SakuraOps", docs_url=None, redoc_url=None)

app.add_middleware(
    SessionMiddleware,
    secret_key=secret,
    session_cookie="sakuraops_session",
    same_site="lax",
    https_only=os.getenv("SAKURAOPS_SECURE_COOKIE", "false").lower() == "true",
    max_age=3600,
)

ph = PasswordHasher()

STYLE = """
<style>
body { background:#111820; color:#e5e7eb; font-family:system-ui;
       max-width:650px; margin:8vh auto; padding:24px; }
h1 {color:#e8b4c8}
input,button {box-sizing:border-box;display:block;width:100%;
padding:12px;margin:12px 0;background:#202c37;color:white;
border:1px solid #526273;border-radius:6px;font:inherit}
button {background:#c47d9c;color:#111820;cursor:pointer;font-weight:bold}
pre {background:#202c37;padding:20px;border-radius:6px}
small {color:#aebbc7}
</style>
"""


def page(content):
    return HTMLResponse(
        "<!doctype html><html lang='pl'><meta charset='utf-8'>"
        "<meta name='viewport' content='width=device-width,initial-scale=1'>"
        "<title>SakuraOps</title>"
        + STYLE + "<body>" + content + "</body></html>"
    )


def current_user(request):
    user_id = request.session.get("user_id")
    if not isinstance(user_id, int):
        return None

    with SessionLocal() as db:
        user = db.get(User, user_id)
        if not user or not user.active:
            return None
        return {"username": user.username, "role": user.role}


@app.get("/")
def home(request: Request):
    if not current_user(request):
        return RedirectResponse("/login", status_code=303)

    user = current_user(request)
    csrf = request.session.setdefault("csrf", secrets.token_urlsafe(32))

    return page(
        "<h1>🌸 SakuraOps</h1>"
        "<p>Zalogowano jako: <b>" + html.escape(user["username"]) + "</b></p>"
        "<p>Rola: " + html.escape(user["role"]) + "</p>"
        "<h2>Konsola administracyjna</h2>"
        "<pre>skr status\nskr server list\nskr help</pre>"
        "<small>Podgląd przyszłych poleceń. Konsola nie jest jeszcze aktywna.</small>"
        "<form method='post' action='/logout'>"
        "<input type='hidden' name='csrf' value='" + csrf + "'>"
        "<button>Wyloguj</button></form>"
    )


@app.get("/login")
def login_page(request: Request):
    if current_user(request):
        return RedirectResponse("/", status_code=303)

    csrf = secrets.token_urlsafe(32)
    request.session["csrf"] = csrf

    return page(
        "<h1>🌸 SakuraOps</h1>"
        "<p>Zaloguj się do panelu administracyjnego</p>"
        "<form method='post' action='/login'>"
        "<input type='hidden' name='csrf' value='" + csrf + "'>"
        "<input name='username' placeholder='Login' autocomplete='username' required>"
        "<input name='password' type='password' placeholder='Hasło' "
        "autocomplete='current-password' required>"
        "<button>Zaloguj lokalnie</button></form>"
        "<small>SSO: planowana integracja opcjonalna</small>"
    )


@app.post("/login")
def login(
    request: Request,
    username: str = Form(),
    password: str = Form(),
    csrf: str = Form(),
):
    expected = request.session.get("csrf", "")

    if not expected or not hmac.compare_digest(csrf, expected):
        raise HTTPException(403, "Nieprawidłowy token formularza")

    with SessionLocal() as db:
        user = db.scalar(select(User).where(User.username == username))

        valid = False
        if user and user.active:
            try:
                valid = ph.verify(user.password_hash, password)
            except VerificationError:
                pass

        if not valid:
            return HTMLResponse(
                "<h2>Nieprawidłowy login lub hasło</h2>"
                "<p><a href='/login'>Spróbuj ponownie</a></p>",
                status_code=401,
            )

        request.session.clear()
        request.session["user_id"] = user.id
        request.session["csrf"] = secrets.token_urlsafe(32)

    return RedirectResponse("/", status_code=303)


@app.post("/logout")
def logout(request: Request, csrf: str = Form()):
    expected = request.session.get("csrf", "")
    if not expected or not hmac.compare_digest(csrf, expected):
        raise HTTPException(403, "Nieprawidłowy token formularza")

    request.session.clear()
    return RedirectResponse("/login", status_code=303)


@app.get("/api/me")
def me(request: Request):
    user = current_user(request)
    if not user:
        raise HTTPException(401, "Wymagane logowanie")
    return user

# SakuraOps Console v0.1
import json
from pydantic import BaseModel, Field
from .commands import execute


class ConsoleCommand(BaseModel):
    command: str = Field(min_length=1, max_length=500)


@app.get("/console")
def console_page(request: Request):
    if not current_user(request):
        return RedirectResponse("/login", status_code=303)

    csrf = request.session.setdefault("csrf", secrets.token_urlsafe(32))
    template = (
        Path(__file__).resolve().parent / "static" / "console.html"
    ).read_text(encoding="utf-8")

    script = (
        "<script>window.SAKURAOPS_CSRF = "
        + json.dumps(csrf)
        + ";</script>"
    )
    template = template.replace("</head>", script + "</head>")
    return HTMLResponse(template)


@app.post("/api/console")
def console_execute(request: Request, body: ConsoleCommand):
    user = current_user(request)
    if not user:
        raise HTTPException(401, "Wymagane logowanie")

    expected = request.session.get("csrf", "")
    supplied = request.headers.get("X-CSRF-Token", "")

    if not expected or not hmac.compare_digest(expected, supplied):
        raise HTTPException(403, "Nieprawidłowy token formularza")

    return {"output": execute(body.command)}

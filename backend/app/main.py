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

    csrf = request.session.setdefault("csrf", secrets.token_urlsafe(32))
    template_path = Path(__file__).resolve().parent / "static" / "dashboard.html"
    template = template_path.read_text(encoding="utf-8")
    token = html.escape(csrf, quote=True)
    template = template.replace(
        "</head>",
        f'<meta name="sakuraops-csrf" content="{token}"></head>',
    )
    return HTMLResponse(template, headers={"Cache-Control": "no-store"})


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
from pydantic import BaseModel, Field
from .commands import interpret, complete
from .workspace import list_files, read_text, write_text


class ConsoleCommand(BaseModel):
    command: str = Field(min_length=1, max_length=500)


@app.get("/console")
def console_page(request: Request):
    if not current_user(request):
        return RedirectResponse("/login", status_code=303)
    return RedirectResponse("/?terminal=1", status_code=303)


@app.post("/api/console")
def console_execute(request: Request, body: ConsoleCommand):
    user = current_user(request)
    if not user:
        raise HTTPException(401, "Wymagane logowanie")

    expected = request.session.get("csrf", "")
    supplied = request.headers.get("X-CSRF-Token", "")

    if not expected or not hmac.compare_digest(expected, supplied):
        raise HTTPException(403, "Nieprawidłowy token formularza")

    mode = request.session.get("console_mode", "exec")
    result = interpret(body.command, mode)
    request.session["console_mode"] = result["mode"]
    return result

class CompletionRequest(BaseModel):
    prefix: str = Field(max_length=500)


@app.post("/api/console/complete")
def console_complete(request: Request, body: CompletionRequest):
    require_console_session(request)
    mode = request.session.get("console_mode", "exec")
    return {"matches": complete(body.prefix, mode)}


def require_console_session(request: Request, *, edit: bool = False):
    user = current_user(request)
    if user is None:
        raise HTTPException(401, "Wymagane logowanie")
    expected = request.session.get("csrf", "")
    supplied = request.headers.get("X-CSRF-Token", "")
    if not expected or not hmac.compare_digest(expected, supplied):
        raise HTTPException(403, "Nieprawidłowy CSRF")
    if edit and user["role"] != "admin":
        raise HTTPException(403, "Edycja plikow wymaga roli administratora")
    return user


class WorkspaceWrite(BaseModel):
    content: str = Field(max_length=65536)


@app.get("/api/workspace")
def workspace_files(request: Request):
    require_console_session(request)
    return {"files": list_files()}


@app.get("/api/workspace/{filename}")
def workspace_read(request: Request, filename: str):
    require_console_session(request)
    return {"filename": filename, "content": read_text(filename)}


@app.put("/api/workspace/{filename}")
def workspace_write(request: Request, filename: str, body: WorkspaceWrite):
    require_console_session(request, edit=True)
    write_text(filename, body.content)
    return {"saved": True, "filename": filename}

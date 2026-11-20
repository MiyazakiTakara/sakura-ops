import asyncio
import hmac
import html
import json
import os
import secrets
from contextlib import asynccontextmanager
from pathlib import Path

from argon2 import PasswordHasher
from argon2.exceptions import VerificationError
from fastapi import FastAPI, Form, HTTPException, Request
from fastapi.responses import HTMLResponse, RedirectResponse, StreamingResponse
from fastapi.staticfiles import StaticFiles
from sqlalchemy import select
from starlette.middleware.sessions import SessionMiddleware

from .database import SessionLocal
from .models import User
from . import ssh_diagnostics, ansible_ping

secret_file = Path(__file__).resolve().parents[2] / ".secrets/session_key"
secret = secret_file.read_text().strip()

@asynccontextmanager
async def app_lifespan(_app: FastAPI):
    from .local_jobs import start_worker, stop_worker
    start_worker()
    ssh_diagnostics.start_worker()
    ansible_ping.start_worker()
    try:
        yield
    finally:
        ansible_ping.stop_worker()
        ssh_diagnostics.stop_worker()
        stop_worker()


app = FastAPI(title="SakuraOps", docs_url=None, redoc_url=None, lifespan=app_lifespan)
app.mount("/static", StaticFiles(directory=Path(__file__).resolve().parent / "static"), name="static")

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
from .console_completion import context_complete
from .workspace import list_files, read_text, write_text
from .inventory import discover
from .repositories import list_repositories, add_repository, delete_repository, repository_fingerprint
from .git_sync import synchronize, list_cached_files, read_repository_file, write_repository_file, diff_repository_file
from .git_changes import git_status, commit_selected, push_explicit
from .inventory_preview import preview_inventory
from .task_plans import task_plan
from . import preflight
from .local_jobs import (enqueue, list_jobs, get_job, cancel_job,
                         enabled as local_jobs_enabled, start_worker, stop_worker)


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
    if result.get("action") == "repo-list":
        if user["role"] != "admin":
            raise HTTPException(403, "Inventory wymaga roli administratora")
        heading, paths = discover(result["kind"])
        result["output"] = heading + ("\\n" + "\n".join(paths) if paths else "")
        result.pop("action", None)
    action = result.get("action")
    if action in ("ssh-diagnostic-create", "ssh-diagnostic-list",
                  "ssh-diagnostic-show", "ssh-diagnostic-cancel"):
        if user["role"] != "admin":
            raise HTTPException(403, "Diagnostyka SSH wymaga roli administratora")
        if action == "ssh-diagnostic-create":
            job = ssh_diagnostics.enqueue(result["alias"])
            result["output"] = "Zlecono diagnostykę SSH #" + str(job["id"]) + " (" + job["alias"] + ")"
        elif action == "ssh-diagnostic-list":
            jobs = ssh_diagnostics.list_jobs()
            result["output"] = ("\n".join(
                "#" + str(job["id"]) + "  " + job["alias"] + "  " + job["status"]
                for job in jobs
            ) or "Brak diagnostyk SSH.")
        elif action == "ssh-diagnostic-show":
            job = ssh_diagnostics.get_job(result["job_id"])
            result["output"] = "SSH #" + str(job["id"]) + " — " + job["status"] + "\n" + job["log"]
        else:
            job = ssh_diagnostics.cancel_job(result["job_id"])
            result["output"] = "SSH #" + str(job["id"]) + ": " + job["status"]
        for field in ("action", "alias", "job_id"):
            result.pop(field, None)
        return result
    if action in ("local-test-create", "local-test-list", "local-test-show", "local-test-cancel"):
        if user["role"] != "admin":
            raise HTTPException(403, "Zadania wymagają roli administratora")
        if action == "local-test-create":
            row = enqueue()
            result["output"] = "Test localhost dodany do kolejki: #" + str(row["id"])
        elif action == "local-test-list":
            rows = list_jobs()
            result["output"] = ("\n".join(
                "#" + str(row["id"]) + "  " + row["status"] + "  " + row["created_at"]
                for row in rows
            ) or "Brak zadań.")
        elif action == "local-test-show":
            row = get_job(result["job_id"])
            result["output"] = ("Test #" + str(row["id"]) + " — " + row["status"]
                                + "\n" + row["log"])
        else:
            row = cancel_job(result["job_id"])
            result["output"] = "Test #" + str(row["id"]) + ": " + row["status"]
        result.pop("action", None)
        result.pop("job_id", None)
        return result

    if action in ("repositories-list", "repository-add", "repository-delete", "repository-sync", "repository-files", "repository-read", "repository-edit", "repository-diff", "repository-status", "repository-commit", "repository-push", "inventory-preview", "task-plan"):
        if user["role"] != "admin":
            raise HTTPException(403, "Zarządzanie repozytoriami wymaga roli administratora")
        if action == "task-plan":
            plan = task_plan(result["name"], result["inventory"], result["playbook"], result["group"])
            result["output"] = ("PLAN (bez wykonania)\nRepo: " + plan["repository"]
                                + "\nInventory: " + plan["inventory"]
                                + "\nPlaybook: " + plan["playbook"]
                                + "\nGrupa: " + plan["limit"]
                                + "\nHosty (" + str(plan["host_count"]) + "): "
                                + ", ".join(plan["hosts"]))
        elif action == "inventory-preview":
            preview = preview_inventory(result["name"], result["filename"])
            if result["kind"] == "hosts":
                result["output"] = "\n".join(preview["hosts"]) or "Brak hostów."
            else:
                result["output"] = "\n".join(
                    row["name"] + " (" + str(row["host_count"]) + "): "
                    + (", ".join(row["hosts"]) or "brak hostów")
                    for row in preview["groups"]
                ) or "Brak grup."
        elif action == "repositories-list":
            repositories = list_repositories()
            result["output"] = ("\\n".join(
                f"{row['name']}  {row['branch']}  {row['url']}  (bez synchronizacji)"
                for row in repositories
            ) or "Brak skonfigurowanych repozytoriów.")
        elif action == "repository-add":
            row = add_repository(result["name"], result["url"], result["branch"])
            result["output"] = f"Zapisano repozytorium {row['name']} (bez synchronizacji Git)."
        elif action == "repository-sync":
            state = synchronize(result["name"])
            result["output"] = f"Synchronizacja zakończona: {state['commit_sha'][:12]}"
        elif action == "repository-status":
            status = git_status(result["name"])
            result["output"] = ("Gałąź: " + status["branch"] + "  HEAD: " + status["head"]
                                + "\n" + ("Zmodyfikowane:\n" + "\n".join(status["modified_files"])
                                           if status["modified_files"] else "Brak plików do commita."))
        elif action == "repository-commit":
            entry = commit_selected(result["name"], result["files"],
                                    result["commit_message"], result["expected_head"])
            result["output"] = "Commit lokalny: " + entry["commit_sha"] + " (bez push)"
        elif action == "repository-push":
            pushed = push_explicit(result["name"], result["expected_head"])
            result["output"] = "Wysłano " + pushed["pushed_sha"] + " do origin/" + pushed["branch"]
        elif action == "repository-diff":
            result["output"] = diff_repository_file(result["name"], result["filename"])["diff"] or "Brak lokalnych zmian."
        elif action == "repository-read":
            file = read_repository_file(result["name"], result["filename"])
            result["output"] = file["content"]
        elif action == "repository-edit":
            read_repository_file(result["name"], result["filename"])
            result["action"] = "repository-edit"
            result["output"] = ""
            return result
        elif action == "repository-files":
            paths = list_cached_files(result["name"], result["kind"])
            result["output"] = "\n".join(paths) if paths else "Brak pasujących plików."
        else:
            delete_repository(result["name"])
            result["output"] = f"Usunięto konfigurację {result['name']}."
        for field in ("action", "name", "url", "branch", "kind", "filename", "files", "commit_message", "expected_head"):
            result.pop(field, None)
    return result

class CompletionRequest(BaseModel):
    prefix: str = Field(max_length=500)


@app.post("/api/console/complete")
def console_complete(request: Request, body: CompletionRequest):
    user = require_console_session(request)
    mode = request.session.get("console_mode", "exec")
    if user["role"] != "admin":
        return {"matches": complete(body.prefix, mode)}
    matches, hint = context_complete(body.prefix, mode)
    return {"matches": matches, "hint": hint}


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


class RepositoryInput(BaseModel):
    name: str = Field(min_length=2, max_length=40)
    url: str = Field(min_length=10, max_length=500)
    branch: str = Field(default="main", min_length=1, max_length=100)


@app.get("/api/repositories")
def repositories_get(request: Request):
    user = require_console_session(request)
    if user["role"] != "admin":
        raise HTTPException(403, "Dostęp wymaga roli administratora")
    return {"repositories": list_repositories()}


@app.post("/api/repositories", status_code=201)
def repositories_post(request: Request, body: RepositoryInput):
    require_console_session(request, edit=True)
    return add_repository(body.name, body.url, body.branch)


@app.post("/api/repositories/{name}/sync")
def repository_sync(request: Request, name: str):
    require_console_session(request, edit=True)
    return synchronize(name)


@app.get("/api/repositories/{name}/files")
def repository_files(request: Request, name: str, kind: str = "inventory"):
    require_console_session(request, edit=True)
    return {"name": name, "kind": kind, "files": list_cached_files(name, kind)}


class RepositoryFileWrite(BaseModel):
    content: str = Field(max_length=65536)
    revision: str = Field(min_length=64, max_length=64)


@app.get("/api/repositories/{name}/content")
def repository_content_get(request: Request, name: str, path: str):
    require_console_session(request, edit=True)
    return read_repository_file(name, path)


@app.put("/api/repositories/{name}/content")
def repository_content_put(request: Request, name: str, path: str, body: RepositoryFileWrite):
    require_console_session(request, edit=True)
    return write_repository_file(name, path, body.content, body.revision)


@app.get("/api/repositories/{name}/diff")
def repository_diff(request: Request, name: str, path: str):
    require_console_session(request, edit=True)
    return diff_repository_file(name, path)


class GitCommitRequest(BaseModel):
    files: list[str] = Field(min_length=1, max_length=30)
    message: str = Field(min_length=1, max_length=200)
    expected_head: str = Field(min_length=40, max_length=40)


class GitPushRequest(BaseModel):
    expected_head: str = Field(min_length=40, max_length=40)


@app.get("/api/repositories/{name}/git-status")
def repository_git_status(request: Request, name: str):
    require_console_session(request, edit=True)
    return git_status(name)


@app.post("/api/repositories/{name}/commit")
def repository_commit(request: Request, name: str, body: GitCommitRequest):
    require_console_session(request, edit=True)
    return commit_selected(name, body.files, body.message, body.expected_head)


@app.post("/api/repositories/{name}/push")
def repository_push(request: Request, name: str, body: GitPushRequest):
    require_console_session(request, edit=True)
    return push_explicit(name, body.expected_head)


@app.get("/api/repositories/{name}/inventory-preview")
def repository_inventory_preview(request: Request, name: str, path: str):
    require_console_session(request, edit=True)
    return preview_inventory(name, path)


class TaskPlanRequest(BaseModel):
    repository: str = Field(min_length=2, max_length=40)
    inventory: str = Field(min_length=1, max_length=500)
    playbook: str = Field(min_length=1, max_length=500)
    group: str = Field(min_length=1, max_length=255)


@app.post("/api/tasks/plan")
def preview_task_plan(request: Request, body: TaskPlanRequest):
    require_console_session(request, edit=True)
    return task_plan(body.repository, body.inventory, body.playbook, body.group)


class PreflightRequest(TaskPlanRequest):
    mode: str = Field(default="diagnostic", max_length=20)


class PreflightApproval(BaseModel):
    fingerprint: str = Field(min_length=64, max_length=64)


@app.post("/api/preflight/reviews", status_code=201)
def preflight_create(request: Request, body: PreflightRequest):
    user = require_console_session(request, edit=True)
    return preflight.create(body.repository, body.inventory, body.playbook,
                            body.group, body.mode, user["username"])


@app.get("/api/preflight/reviews")
def preflight_recent(request: Request):
    require_console_session(request, edit=True)
    return {"reviews": preflight.recent()}


@app.get("/api/preflight/reviews/{review_id}")
def preflight_get(request: Request, review_id: int):
    require_console_session(request, edit=True)
    return preflight.get(review_id)


@app.post("/api/preflight/reviews/{review_id}/approve")
def preflight_approve(request: Request, review_id: int, body: PreflightApproval):
    user = require_console_session(request, edit=True)
    return preflight.approve(review_id, body.fingerprint, user["username"])


@app.get("/api/jobs")
def jobs_get(request: Request):
    require_console_session(request, edit=True)
    return {"enabled": local_jobs_enabled(), "jobs": list_jobs()}


@app.post("/api/jobs/local-test", status_code=201)
def jobs_local_test(request: Request):
    require_console_session(request, edit=True)
    return enqueue()


@app.get("/api/jobs/{job_id}")
def jobs_get_one(request: Request, job_id: int):
    require_console_session(request, edit=True)
    return get_job(job_id)


@app.post("/api/jobs/{job_id}/cancel")
def jobs_cancel(request: Request, job_id: int):
    require_console_session(request, edit=True)
    return cancel_job(job_id)


@app.get("/api/jobs/{job_id}/events")
async def jobs_events(request: Request, job_id: int):
    # EventSource cannot attach X-CSRF-Token; this endpoint is read-only.
    user = current_user(request)
    if user is None:
        raise HTTPException(401, "Wymagane logowanie")
    if user["role"] != "admin":
        raise HTTPException(403, "Wymagana rola administratora")
    get_job(job_id)

    async def updates():
        previous = None
        while not await request.is_disconnected():
            account = await asyncio.to_thread(current_user, request)
            if account is None or account["role"] != "admin":
                return
            row = await asyncio.to_thread(get_job, job_id)
            signature = (row["status"], row["log"], row["cancel_requested"])
            if signature != previous:
                previous = signature
                yield "event: job-update\ndata: " + json.dumps(
                    row, ensure_ascii=False
                ) + "\n\n"
            if row["status"] in ("success", "failed", "cancelled"):
                return
            await asyncio.sleep(0.5)
    return StreamingResponse(
        updates(), media_type="text/event-stream",
        headers={"Cache-Control": "no-store", "X-Accel-Buffering": "no"},
    )


class SSHDiagnosticRequest(BaseModel):
    alias: str = Field(min_length=2, max_length=40)


@app.get("/api/ssh-diagnostics/config")
def ssh_diagnostics_config(request: Request):
    require_console_session(request, edit=True)
    return ssh_diagnostics.configuration()


@app.get("/api/ssh-diagnostics/jobs")
def ssh_diagnostics_jobs(request: Request):
    require_console_session(request, edit=True)
    return {"jobs": ssh_diagnostics.list_jobs()}


@app.post("/api/ssh-diagnostics/jobs", status_code=201)
def ssh_diagnostics_enqueue(request: Request, body: SSHDiagnosticRequest):
    require_console_session(request, edit=True)
    return ssh_diagnostics.enqueue(body.alias)


@app.get("/api/ssh-diagnostics/jobs/{job_id}")
def ssh_diagnostics_get(request: Request, job_id: int):
    require_console_session(request, edit=True)
    return ssh_diagnostics.get_job(job_id)


@app.post("/api/ssh-diagnostics/jobs/{job_id}/cancel")
def ssh_diagnostics_cancel(request: Request, job_id: int):
    require_console_session(request, edit=True)
    return ssh_diagnostics.cancel_job(job_id)


@app.get("/api/ssh-diagnostics/jobs/{job_id}/events")
async def ssh_diagnostics_events(request: Request, job_id: int):
    user = current_user(request)
    if user is None:
        raise HTTPException(401, "Wymagane logowanie")
    if user["role"] != "admin":
        raise HTTPException(403, "Wymagana rola administratora")
    ssh_diagnostics.get_job(job_id)

    async def updates():
        previous = None
        while not await request.is_disconnected():
            account = await asyncio.to_thread(current_user, request)
            if account is None or account["role"] != "admin":
                return
            row = await asyncio.to_thread(ssh_diagnostics.get_job, job_id)
            signature = (row["status"], row["log"], row["cancel_requested"])
            if signature != previous:
                previous = signature
                yield "event: job-update\ndata: " + json.dumps(
                    row, ensure_ascii=False
                ) + "\n\n"
            if row["status"] in ("success", "failed", "cancelled"):
                return
            await asyncio.sleep(0.5)
    return StreamingResponse(updates(), media_type="text/event-stream",
                             headers={"Cache-Control": "no-store", "X-Accel-Buffering": "no"})


class AnsiblePingRequest(BaseModel):
    alias: str = Field(min_length=2, max_length=40)


@app.get("/api/ansible-ping/config")
def ping_config(request: Request):
    require_console_session(request, edit=True)
    return ansible_ping.configuration()


@app.get("/api/ansible-ping/jobs")
def ping_jobs(request: Request):
    require_console_session(request, edit=True)
    return {"jobs": ansible_ping.list_jobs()}


@app.post("/api/ansible-ping/jobs", status_code=201)
def ping_enqueue(request: Request, body: AnsiblePingRequest):
    require_console_session(request, edit=True)
    return ansible_ping.enqueue(body.alias)


@app.get("/api/ansible-ping/jobs/{job_id}")
def ping_job(request: Request, job_id: int):
    require_console_session(request, edit=True)
    return ansible_ping.get_job(job_id)


@app.post("/api/ansible-ping/jobs/{job_id}/cancel")
def ping_cancel(request: Request, job_id: int):
    require_console_session(request, edit=True)
    return ansible_ping.cancel(job_id)


@app.get("/api/ansible-ping/jobs/{job_id}/events")
async def ping_events(request: Request, job_id: int):
    user = current_user(request)
    if user is None:
        raise HTTPException(401, "Wymagane logowanie")
    if user["role"] != "admin":
        raise HTTPException(403, "Wymagana rola administratora")
    ansible_ping.get_job(job_id)

    async def updates():
        previous = None
        while not await request.is_disconnected():
            account = await asyncio.to_thread(current_user, request)
            if account is None or account["role"] != "admin":
                return
            row = await asyncio.to_thread(ansible_ping.get_job, job_id)
            signature = (row["status"], row["log"], row["cancel_requested"])
            if signature != previous:
                previous = signature
                yield "event: job-update\ndata: " + json.dumps(row, ensure_ascii=False) + "\n\n"
            if row["status"] in ("success", "failed", "cancelled"):
                return
            await asyncio.sleep(0.5)

    return StreamingResponse(updates(), media_type="text/event-stream",
                             headers={"Cache-Control": "no-store", "X-Accel-Buffering": "no"})


@app.delete("/api/repositories/{name}")
def repositories_delete(request: Request, name: str):
    require_console_session(request, edit=True)
    delete_repository(name)
    return {"deleted": name}


@app.get("/api/repositories/events")
async def repository_events(request: Request):
    user = current_user(request)
    if user is None:
        raise HTTPException(401, "Wymagane logowanie")
    if user["role"] != "admin":
        raise HTTPException(403, "Wymagana rola administratora")

    async def stream():
        previous = await asyncio.to_thread(repository_fingerprint)
        # Initial handshake does not force a redundant list refresh.
        yield "event: repositories-changed\ndata: {}\nretry: 2000\n\n"
        elapsed = 0
        while not await request.is_disconnected():
            await asyncio.sleep(1)
            elapsed += 1
            # Check the current session; stop streaming if account is disabled.
            if elapsed % 15 == 0:
                account = await asyncio.to_thread(current_user, request)
                if account is None or account["role"] != "admin":
                    break
            current = await asyncio.to_thread(repository_fingerprint)
            if current != previous:
                previous = current
                yield "event: repositories-changed\ndata: {}\n\n"
            elif elapsed % 15 == 0:
                yield ": heartbeat\n\n"

    return StreamingResponse(
        stream(),
        media_type="text/event-stream",
        headers={"Cache-Control": "no-store", "X-Accel-Buffering": "no"},
    )

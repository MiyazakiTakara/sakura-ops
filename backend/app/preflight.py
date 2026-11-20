"""Static preflight. No Ansible execution, no remote connection."""
import hashlib
import json
import os
import re
from datetime import datetime, timezone
from fastapi import HTTPException
from sqlalchemy import select
from .database import SessionLocal, engine
from .models import PreflightReview
from .git_sync import SHA, _git, _location, list_cached_files, read_repository_file, synchronization_status
from .inventory_preview import preview_inventory

PreflightReview.__table__.create(bind=engine, checkfirst=True)
IDENT = re.compile(r"[A-Za-z0-9_.:@-]{1,255}\Z")
MODULES = {"ansible.builtin.debug", "ansible.builtin.ping"}


def _now():
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


def _allowlist():
    names = {x.strip() for x in os.getenv("SAKURAOPS_PREFLIGHT_HOSTS", "").split(",") if x.strip()}
    if len(names) > 100 or any(not IDENT.fullmatch(x) for x in names):
        raise HTTPException(503, "Nieprawidłowa allowlista hostów")
    return names


def _snapshot(repo, inventory, playbook):
    if inventory not in list_cached_files(repo, "inventory"):
        raise HTTPException(404, "Inventory nie jest śledzone w Git")
    if playbook not in list_cached_files(repo, "playbooks"):
        raise HTTPException(404, "Playbook nie jest śledzony w Git")
    root = str(_location(repo))
    sha = _git(["-C", root, "rev-parse", "--verify", "HEAD"])
    if not SHA.fullmatch(sha) or sha != synchronization_status(repo)["commit_sha"]:
        raise HTTPException(409, "Git HEAD nie odpowiada synchronizacji")
    if _git(["-C", root, "status", "--porcelain", "--untracked-files=no"]):
        raise HTTPException(409, "Repozytorium zawiera niezacommitowane zmiany")
    inv = read_repository_file(repo, inventory)
    play = read_repository_file(repo, playbook)
    if (_git(["-C", root, "rev-parse", "--verify", "HEAD"]) != sha or
            _git(["-C", root, "status", "--porcelain", "--untracked-files=no"])):
        raise HTTPException(409, "Git zmienił się podczas analizy")
    return sha, inv, play


def _parse(source):
    import yaml
    try:
        if any(isinstance(event, yaml.AliasEvent) or getattr(event, "anchor", None)
               for event in yaml.parse(source)):
            return [], ["Kotwice i aliasy YAML są niedozwolone"]
        plays = yaml.safe_load(source)
    except yaml.YAMLError:
        return [], ["Błąd składni YAML"]
    if not isinstance(plays, list) or not 1 <= len(plays) <= 10:
        return [], ["Wymagane 1–10 play"]
    modules, errors, count = [], [], 0
    for i, play in enumerate(plays, 1):
        if not isinstance(play, dict):
            errors.append(f"Play {i}: nieprawidłowy format")
            continue
        if set(play) - {"name", "hosts", "gather_facts", "tasks"}:
            errors.append(f"Play {i}: nieobsługiwane pola")
        if play.get("hosts") != "all" or play.get("gather_facts") is not False:
            errors.append(f"Play {i}: wymagane hosts: all i gather_facts: false")
        tasks = play.get("tasks")
        if not isinstance(tasks, list) or not tasks:
            errors.append(f"Play {i}: brak listy zadań")
            continue
        count += len(tasks)
        if count > 40:
            errors.append("Limit 40 zadań")
            break
        for j, task in enumerate(tasks, 1):
            if not isinstance(task, dict):
                errors.append(f"Play {i}, zadanie {j}: wymagany obiekt")
                continue
            found = set(task) & MODULES
            if len(found) != 1 or set(task) - MODULES - {"name"}:
                errors.append(f"Play {i}, zadanie {j}: niedozwolony moduł lub dyrektywa")
                continue
            if "name" in task and (not isinstance(task["name"], str) or len(task["name"]) > 120):
                errors.append(f"Play {i}, zadanie {j}: niepoprawna nazwa")
                continue
            module = next(iter(found))
            args = task[module]
            if module == "ansible.builtin.ping" and args not in (None, {}):
                errors.append(f"Play {i}, zadanie {j}: ping bez argumentów")
                continue
            if module == "ansible.builtin.debug":
                if not isinstance(args, dict) or set(args) != {"msg"} or not isinstance(args["msg"], str):
                    errors.append(f"Play {i}, zadanie {j}: debug wymaga literalnego msg")
                    continue
                message = args["msg"]
                if not message.isascii() or len(message) > 160 or any(
                    char in message for char in ("{", "}", "$", "%", chr(96))
                ):
                    errors.append(f"Play {i}, zadanie {j}: niedozwolone wyrażenie")
                    continue
            modules.append(module)
    return modules, errors[:30]


def _assess(repo, inventory, playbook, group, mode):
    if mode not in ("diagnostic", "check") or not IDENT.fullmatch(group):
        raise HTTPException(422, "Nieprawidłowy tryb lub grupa")
    sha, inv, play = _snapshot(repo, inventory, playbook)
    inventory_data = preview_inventory(repo, inventory)
    selected = next((g for g in inventory_data["groups"] if g["name"] == group), None)
    if selected is None or not selected["hosts"]:
        raise HTTPException(422, "Brak hostów w wybranej grupie")
    hosts = selected["hosts"]
    errors = []
    if len(hosts) > 20:
        errors.append("Limit 20 hostów")
    unknown = sorted(set(hosts) - _allowlist())
    if unknown:
        errors.append("Hosty spoza allowlisty: " + ", ".join(unknown[:10]))
    if not playbook.endswith((".yml", ".yaml")):
        errors.append("Playbook musi być plikiem YAML")
    modules, problems = _parse(play["content"])
    errors.extend(problems)
    report = {"repository": repo, "inventory": inventory, "playbook": playbook,
              "group": group, "mode": mode, "git_sha": sha,
              "inventory_digest": inv["revision"], "playbook_digest": play["revision"],
              "hosts": hosts[:20], "host_count": len(hosts),
              "modules": modules, "blockers": errors, "eligible": not errors,
              "executable": False,
              "warning": "Statyczny raport; bez wykonania Ansible i bez połączeń z hostami."}
    raw = json.dumps(report, sort_keys=True, separators=(",", ":"), ensure_ascii=True)
    return report, hashlib.sha256(raw.encode("utf-8")).hexdigest()


def _dto(row):
    return {"id": row.id, "fingerprint": row.fingerprint, "state": row.state,
            "created_by": row.created_by, "created_at": row.created_at,
            "approved_by": row.approved_by, "approved_at": row.approved_at,
            "report": json.loads(row.report_json)}


def create(repo, inventory, playbook, group, mode, actor):
    report, fingerprint = _assess(repo, inventory, playbook, group, mode)
    with SessionLocal() as db:
        item = PreflightReview(
            repository=repo, inventory=inventory, playbook=playbook,
            target_group=group, mode=mode, git_sha=report["git_sha"],
            inventory_digest=report["inventory_digest"],
            playbook_digest=report["playbook_digest"],
            fingerprint=fingerprint, report_json=json.dumps(report, sort_keys=True),
            state="reviewable" if report["eligible"] else "blocked",
            created_by=actor, created_at=_now())
        db.add(item)
        db.commit()
        db.refresh(item)
        return _dto(item)


def get(review_id):
    with SessionLocal() as db:
        item = db.get(PreflightReview, review_id)
        if item is None:
            raise HTTPException(404, "Raport nie istnieje")
        return _dto(item)


def recent():
    with SessionLocal() as db:
        return [_dto(item) for item in db.scalars(
            select(PreflightReview).order_by(PreflightReview.id.desc()).limit(30))]


def approve(review_id, fingerprint, actor):
    if not re.fullmatch(r"[0-9a-f]{64}", fingerprint):
        raise HTTPException(422, "Nieprawidłowy fingerprint")
    with SessionLocal() as db:
        item = db.get(PreflightReview, review_id)
        if item is None:
            raise HTTPException(404, "Raport nie istnieje")
        if item.state != "reviewable" or item.fingerprint != fingerprint:
            raise HTTPException(409, "Plan zablokowany lub fingerprint niezgodny")
        report, current = _assess(
            item.repository, item.inventory, item.playbook, item.target_group, item.mode)
        if not report["eligible"] or current != item.fingerprint:
            raise HTTPException(409, "Plan wygasł. Uruchom preflight ponownie")
        item.state = "approved"
        item.approved_by = actor
        item.approved_at = _now()
        db.commit()
        db.refresh(item)
        return _dto(item)

"""Strict, side-effect-free planning for a fixed Ansible ping diagnostic.

This module cannot execute commands or open network connections. A future
separately reviewed runner may consume a validated plan only after explicit
host-key, SSH credential, privilege and isolation acceptance.
"""
import hmac
import ipaddress
import json
import os
import re
import stat
from pathlib import Path

ALIAS = re.compile(r"[a-z][a-z0-9-]{0,62}\Z")
PORT = 22
PROFILE = "builtin-ping"
FIXED_ALIAS = "semaphore"
FIXED_ADDRESS = "172.24.3.181"
TICKET_KEYS = {"profile", "mode", "hosts", "sha", "fingerprint"}
GIT_SHA = re.compile(r"[0-9a-f]{40}\Z")
FINGERPRINT = re.compile(r"[0-9a-f]{64}\Z")
MAX_TARGETS = 1
ENTRY_KEYS = {"alias", "address", "user", "host_key_sha256"}
USER = re.compile(r"[a-z_][a-z0-9_-]{0,31}\Z")
HOSTKEY = re.compile(r"SHA256:[A-Za-z0-9+/]{43}\Z")


def validate_target(entry):
    """Allow a single numeric address, dedicated unprivileged account and host key pin."""
    if not isinstance(entry, dict) or set(entry) != ENTRY_KEYS:
        raise ValueError("Unknown target configuration")
    alias, address = entry["alias"], entry["address"]
    username, pin = entry["user"], entry["host_key_sha256"]
    if not isinstance(alias, str) or alias != FIXED_ALIAS:
        raise ValueError("Invalid fixed target alias")
    if not isinstance(username, str) or not USER.fullmatch(username) or username == "root":
        raise ValueError("Dedicated non-root SSH user required")
    if not isinstance(pin, str) or not HOSTKEY.fullmatch(pin):
        raise ValueError("Pinned SSH host public-key fingerprint required")
    try:
        if not isinstance(address, str):
            raise ValueError()
        ip = ipaddress.ip_address(address)
    except ValueError as exc:
        raise ValueError("Only literal IP addresses are allowed") from exc
    if (ip.is_loopback or ip.is_unspecified or ip.is_multicast
            or ip.is_link_local or ip.is_reserved or not ip.is_private
            or ip.version != 4):
        raise ValueError("Target must be an explicit private IPv4 address")
    if str(ip) != FIXED_ADDRESS:
        raise ValueError("Only the fixed Semaphore address is permitted")
    return {"alias": alias, "address": str(ip), "user": username,
            "host_key_sha256": pin}


def read_policy(path):
    """Load operator-owned configuration; never load from Git or user input."""
    file = Path(path)
    if not file.is_absolute() or file.is_symlink():
        raise ValueError("Expected an absolute regular policy file")
    info = file.lstat()
    if (not stat.S_ISREG(info.st_mode) or info.st_uid != 0
            or info.st_mode & 0o022 or info.st_size > 4096):
        raise ValueError("Policy must be root-owned and not writable by other users")
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
    fd = os.open(file, flags)
    try:
        after = os.fstat(fd)
        if after.st_dev != info.st_dev or after.st_ino != info.st_ino:
            raise ValueError("Policy changed while opening")
        data = os.read(fd, 4097)
        if len(data) > 4096:
            raise ValueError("Policy exceeds limit")
    finally:
        os.close(fd)
    try:
        policy = json.loads(data)
    except (UnicodeError, ValueError) as exc:
        raise ValueError("Invalid policy JSON") from exc
    if not isinstance(policy, dict) or set(policy) != {"version", "target"} or policy["version"] != 1:
        raise ValueError("Invalid policy version")
    return validate_target(policy["target"])


def plan(ticket_fields, target, *, approved_sha, approved_fingerprint):
    """Plan only after matching a separately approved Git commit and fingerprint.

    Approved values must come from a trusted operator-controlled approval
    record, never copied from the caller's ticket fields.
    The admission daemon is responsible for authenticating the ticket first.
    """
    if not isinstance(ticket_fields, dict) or set(ticket_fields) != TICKET_KEYS:
        raise ValueError("Ticket fields must match exact diagnostic schema")
    if ticket_fields["profile"] != PROFILE or ticket_fields["mode"] != "diagnostic":
        raise ValueError("Only fixed diagnostic ping is permitted")
    if ticket_fields["hosts"] != [FIXED_ALIAS]:
        raise ValueError("Only the fixed Semaphore alias is permitted")
    if not isinstance(target, dict) or set(target) != ENTRY_KEYS:
        raise ValueError("Invalid target")
    validated = validate_target(target)
    sha, fingerprint = ticket_fields["sha"], ticket_fields["fingerprint"]
    for value, pattern in ((sha, GIT_SHA), (approved_sha, GIT_SHA),
                           (fingerprint, FINGERPRINT),
                           (approved_fingerprint, FINGERPRINT)):
        if not isinstance(value, str) or not pattern.fullmatch(value):
            raise ValueError("Invalid or missing approved Git identity")
    if not hmac.compare_digest(sha, approved_sha) or not hmac.compare_digest(
            fingerprint, approved_fingerprint):
        raise ValueError("Ticket Git identity differs from approved values")
    return {"profile": PROFILE, "host": validated["alias"],
            "address": validated["address"], "ssh_user": validated["user"],
            "pinned_host_key": validated["host_key_sha256"],
            "command_execution": False,
            "reason": "Awaiting reviewed runner and manual Semaphore QA"}

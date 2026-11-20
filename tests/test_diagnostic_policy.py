"""No-execution negative tests for fixed Semaphore diagnostics policy."""
import json
from pathlib import Path

import pytest
from backend.app import diagnostic_policy as policy


def target():
    return {"alias": "semaphore", "address": "172.24.3.181", "user": "sakuraops-ping",
            "host_key_sha256": "SHA256:" + "a" * 43}


def ticket():
    return {"profile": "builtin-ping", "mode": "diagnostic",
            "hosts": ["semaphore"], "sha": "a" * 40,
            "fingerprint": "b" * 64}


def make_plan(fields=None, destination=None, **kwargs):
    return policy.plan(
        ticket() if fields is None else fields,
        policy.validate_target(target()) if destination is None else destination,
        approved_sha=kwargs.get("approved_sha", "a" * 40),
        approved_fingerprint=kwargs.get("approved_fingerprint", "b" * 64),
    )


def test_fixed_diagnostic_is_non_executing():
    got = policy.validate_target(target())
    result = make_plan(destination=got)
    assert result["host"] == "semaphore"
    assert result["command_execution"] is False
    assert "Awaiting" in result["reason"]


@pytest.mark.parametrize(("field", "bad"), [
    ("alias", "root@semaphore"), ("alias", "../semaphore"),
    ("address", "localhost"), ("address", "8.8.8.8"),
    ("address", "127.0.0.1"), ("address", "169.254.1.1"),
    ("address", "172.24.3.181;id"),
    ("user", "root"), ("user", "admin;id"),
    ("host_key_sha256", ""), ("host_key_sha256", "SHA256:bad"),
])
def test_reject_dangerous_target_fields(field, bad):
    value = target()
    value[field] = bad
    with pytest.raises(ValueError):
        policy.validate_target(value)


def test_reject_unknown_target_configuration():
    value = target()
    value["ansible_ssh_common_args"] = "-o StrictHostKeyChecking=no"
    with pytest.raises(ValueError, match="Unknown"):
        policy.validate_target(value)


@pytest.mark.parametrize(("field", "bad"), [
    ("hosts", ["glpi"]), ("hosts", ["semaphore", "glpi"]),
    ("mode", "check"), ("profile", "shell"), ("profile", "ansible.builtin.shell"),
])
def test_reject_unapproved_ticket_scope(field, bad):
    value = ticket()
    value[field] = bad
    with pytest.raises(ValueError):
        make_plan(fields=value)


def test_reject_non_root_policy(tmp_path):
    location = tmp_path / "policy.json"
    location.write_text(json.dumps({"version": 1, "target": target()}))
    with pytest.raises(ValueError, match="root-owned"):
        policy.read_policy(location)


def test_no_execution_primitives_in_policy_module():
    source = (Path(__file__).resolve().parents[1] /
              "backend/app/diagnostic_policy.py").read_text()
    assert "import subprocess" not in source
    assert "import paramiko" not in source
    assert "import socket" not in source
    assert "os.system(" not in source
    assert "ansible-playbook" not in source

@pytest.mark.parametrize(("field", "bad"), [
    ("alias", "other"), ("address", "172.24.3.182"),
    ("address", "10.0.0.1"), ("address", "192.168.1.2"),
])
def test_reject_other_private_targets(field, bad):
    value = target()
    value[field] = bad
    with pytest.raises(ValueError):
        policy.validate_target(value)


@pytest.mark.parametrize(("field", "bad"), [
    ("sha", ""), ("sha", "g" * 40), ("sha", "a" * 39),
    ("sha", "A" * 40), ("fingerprint", ""),
    ("fingerprint", "q" * 64), ("fingerprint", "b" * 63),
])
def test_reject_invalid_git_identity(field, bad):
    fields = ticket()
    fields[field] = bad
    with pytest.raises(ValueError):
        make_plan(fields=fields)


def test_reject_unknown_ticket_field():
    fields = ticket()
    fields["ansible_args"] = "--become"
    with pytest.raises(ValueError, match="exact"):
        make_plan(fields=fields)


def test_reject_missing_ticket_field():
    fields = ticket()
    del fields["sha"]
    with pytest.raises(ValueError, match="exact"):
        make_plan(fields=fields)


def test_reject_unapproved_git_values():
    with pytest.raises(ValueError, match="differs"):
        make_plan(approved_sha="c" * 40)
    with pytest.raises(ValueError, match="differs"):
        make_plan(approved_fingerprint="d" * 64)


def test_reject_invalid_approval_record():
    with pytest.raises(ValueError):
        make_plan(approved_sha=None)
    with pytest.raises(ValueError):
        make_plan(approved_fingerprint="not-a-digest")


def test_target_is_revalidated_before_planning():
    destination = target()
    destination["address"] = "172.24.3.182"
    with pytest.raises(ValueError):
        make_plan(destination=destination)


def test_approval_parameters_are_required():
    with pytest.raises(TypeError):
        policy.plan(ticket(), target())

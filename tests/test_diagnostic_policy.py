"""No-execution negative tests for fixed Semaphore diagnostics policy."""
import copy
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


def test_fixed_diagnostic_is_non_executing():
    got = policy.validate_target(target())
    result = policy.plan(ticket(), got)
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
        policy.plan(value, policy.validate_target(target()))


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

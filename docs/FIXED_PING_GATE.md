# Fixed Semaphore ping profile — planning gate only

This stage of the roadmap adds **non-executing** target selection controls
for a future Ansible diagnostic. It does not enable SSH, subprocesses,
remote updates, shell, arbitrary Git playbooks or new endpoints.

The only currently permitted plan is:
- Built-in `builtin-ping` (not a user-supplied playbook);
- Signed ticket, already validated by the admission daemon;
- `mode=diagnostic`, exact hosts list `["semaphore"]`;
- Fixed and enforced address `172.24.3.181` (not merely an example),
  not discovered dynamically from an inventory file;
- A dedicated non-root SSH account (suggested: `sakuraops-ping`);
- A pinned SSH *host public key* fingerprint, verified out of band.

`backend/app/diagnostic_policy.py` validates an operator-managed
root-owned fixed policy file, exactly `172.24.3.181`,
dedicated username and pinned SHA256 host key metadata. It returns only
a static plan with `command_execution: false`. **Pin metadata is not
host-key enforcement**; a future runner must configure and enforce a
trusted `known_hosts` file before any SSH call. Do not mistake
this planning gate for a complete runtime isolation boundary.

Planning also requires an **exact ticket schema**: `profile`, `mode`,
`hosts`, `sha` (40 lowercase hexadecimal characters), and `fingerprint`
(64 lowercase hexadecimal characters). The latter two are compared against
**separately approved**, trusted values passed as `approved_sha` and
`approved_fingerprint`; they MUST NOT be populated by copying ticket
values. Ticket HMAC verification remains the admission daemon's separate
responsibility. The currently unconnected planner cannot authorize execution.

Example **format only** (not an install-ready file; placeholder pin
must be independently verified and replaced):

```json
{
  "version": 1,
  "target": {
    "alias": "semaphore",
    "address": "172.24.3.181",
    "user": "sakuraops-ping",
    "host_key_sha256": "SHA256:REPLACE_WITH_VERIFIED_BASE64_HOST_KEY_DIGEST"
  }
}
```

Acceptance before remote execution is separately reviewed:
1. Verify the exact Semaphore host public key independently against the
   trusted operator record, and provision read-only pinned known_hosts;
2. Confirm the service credential used for SSH has **no sudo**, no
   writable infrastructure automation secrets, no root access and no
   SSH agent forwarding;
3. Use immutable built-in ping playbook, a distinct isolated process
   with network access restricted to the single destination, tightly
   scoped runtime files, fixed Ansible configuration and environment,
   execution timeout, output cap and cancellation semantics;
4. Prove no arbitrary playbook, inventory, module, task options or host
   can be supplied via IPC/API or modified by the untrusted service;
5. Prove existing `admission_only` behavior remains unchanged until
   an explicitly reviewed enable gate is added;
6. Manual WSL -> Semaphore acceptance test must produce
   `ansible.builtin.ping: pong`, `changed=false` and no host changes.

Never replace the current running `/opt/sakuraops-executor` with a
network-capable runner or disable `PrivateNetwork` as part of this
preparation.

# SakuraOps roadmap

SakuraOps is a self-hosted infrastructure management application, independent of Semaphore. Its primary interface is the `skr` administrative command language, available in the web console and, later, as a Linux CLI. A GUI will call the same backend actions.

## Current implementation status (2026-11-20)

- Local administrator authentication (Argon2, SQLite) and signed sessions.
- Web console `skr` with restricted commands, history and Tab completion.
- Git-backed repository configuration, sync, cached inventory and playbooks,
  file editing, local commits and explicit Git push.
- Static inventory preview and non-executing task planning.
- [PR #15](https://github.com/MiyazakiTakara/sakura-ops/pull/15):
  fixed localhost-only Ansible smoke playbook, persistent queue, live logs,
  cancellation; manual `localhost` test passed.
- [PR #16](https://github.com/MiyazakiTakara/sakura-ops/pull/16):
  allowlisted SSH connectivity diagnostic with separate `sakuraops-diag`
  identity and host-key verification; Semaphore `172.24.3.181` test passed.
- [PR #17](https://github.com/MiyazakiTakara/sakura-ops/pull/17):
  fixed `ansible.builtin.ping` with separate `sakuraops-ping` identity,
  live logs and history; real Semaphore result `pong`, `changed=false`.
- This remains a development prototype, **not production-ready**.
  Neither arbitrary repository playbooks nor production updates are enabled.

## Principles
- Local login always remains available; Keycloak SSO is optional.
- GUI, web console, and future `skr` executable use the same validated application services and permission checks.
- The web console is **not** an unrestricted Linux shell.
- Inventory and playbooks are versioned in Git. User credentials, session signing keys, tokens, SSH keys, and runtime databases are excluded from Git.
- Changes to infrastructure require explicit authorization, target scoping, logging, and safeguards against unintended production changes.
- Start read-only; do not restart servers or apply upgrades by default.
- Deploy beside existing Semaphore, and migrate only after acceptance and tested backups.

## Roadmap

### v0.1 - Secure foundation
- Add automated tests for login, logout, CSRF, unauthorized access, and command parsing.
- Harden cookie handling and login throttling. Document safe local dev vs HTTPS production settings.
- Introduce schema migrations and a reproducible dependency manifest.
- Keep the console functional and accessible.

**Acceptance:** tests pass; unauthenticated requests cannot execute commands; no secrets or database files are tracked.

### v0.2 - Optional Keycloak SSO
- OIDC Authorization Code flow with PKCE and validated issuer, state and nonce.
- Local fallback login; explicit account linking, no implicit email-based linking.
- Roles and authorization tests; events for login and admin actions.

**Acceptance:** local login works with SSO enabled, disabled, or temporarily unavailable.

### v0.3 - Command framework and infrastructure inventory
- Shared command handlers for `skr`, REST API and web console.
- `skr server list/info`, `inventory list`, `config show` (read-only first).
- Git-backed Ansible inventory, validation, structured output and command help/completion.
- A separate `skr` executable for Linux using the same backend API.

**Acceptance:** CLI and web console yield equivalent results; unauthorized users cannot mutate inventory.

### v0.4 - Ansible execution and reporting
- Ansible Runner in an isolated execution environment; managed credentials and least privilege.
- Controlled playbook allowlist, dry-run and safe concurrency limits.
- Read-only Ubuntu update audit, task logs, run history, unreachable-host handling.
- Initial GLPI integration using existing playbooks/inventory.

**Acceptance:** GLPI inventory and update audit are reproducible, with zero host changes.

### v0.5 - Schedules and controlled updates
- Timezone-aware scheduled audits with job locking and retry/error visibility.
- Approval-gated updates; maintenance windows and reboot policy.
- Expand audited platforms to additional Linux distributions and, separately, Windows.

**Acceptance:** no unattended production package installation or reboot without configured authorization.

### v0.6 - Operational GUI
- Server directory, filters and groups, update summaries, job execution and history.
- Forms call the same API as the `skr` command handlers.
- Accessible responsive UI, loading/error/empty states and audited user actions.

**Acceptance:** GUI and CLI are consistent, permission-gated and tested.

### v1.0 - Deployment and operational readiness
- Docker Compose, HTTPS reverse proxy, configuration/secrets documentation.
- Backup/restore drill, upgrade and rollback procedures, role review, logging and monitoring.
- Pilot then planned migration from Semaphore only after operational sign-off.

**Acceptance:** installation, recovery, security and functional tests documented and repeatable.

## Immediate delivery plan — PR #18–21

These milestones form the next development cycle. They can be developed
during one work session, but must remain separate PRs with distinct
tests, risk reviews and acceptance gates. The milestone numbers are
planning labels, not permission to automatically run or merge code.

### PR #18 — Preflight and explicit approval

- Select Git repository, inventory, playbook, target group and requested mode.
- Pin the exact Git commit SHA and reject changes between planning and approval.
- Validate static inventory scope, paths, symlinks, syntax and an explicit
  allowlist of supported modules/operations. Refuse dynamic inventory,
  unknown plugins and unconstrained variables until separately reviewed.
- Show a clear report of hosts, intended operations, risk, blockers and
  authorizing administrator.
- **Acceptance:** automated negative/security cases pass, and **no remote
  playbook execution** occurs at this stage.

### PR #19 — Isolated execution service

- Separate execution service as a least-privileged OS user or container;
  the web API never runs arbitrary shell commands.
- Pass only validated, approved job data to the execution service.
- Keep SSH keys and credentials outside Git, frontend and logs; pin host keys.
- Limit resources, timeout, concurrent execution, cancel behavior, retries
  and crash recovery, including exclusive locks per target.
- **Acceptance:** verify isolation, permissions, secret handling and
  fail-closed behavior before enabling remote execution.

### PR #20 — Approved read-only diagnostic playbooks

- Execute only explicitly reviewed diagnostic playbooks at a pinned SHA.
- Start with Semaphore and a low-privilege identity without `sudo` or
  `become`; other hosts remain disabled until separately approved.
- Candidate diagnostics: OS information, service state, resources and
  available security updates without installing packages.
- Treat `--check` as an extra signal, **not a guarantee of no side effects**.
  Enforce an operation-level allowlist, not just Ansible check mode.
- **Acceptance:** CI and manual Semaphore QA pass; no configuration changes
  and no production host access.

### PR #21 — Audit and reporting

- Record actor, approval, timestamp, exact Git SHA, inventory, targets,
  mode, execution outcome and preflight reference.
- Preserve per-host results, redact credentials and provide exportable
  audit reports with retention rules.
- **Acceptance:** authorization, secret redaction, event integrity and
  restart/recovery cases are tested.

### PR #22 — Schedules and approved templates

- Timezone-aware recurring tasks with locking, clear failure/retry status.
- Only previously approved templates may run; scheduling may not bypass
  preflight or role checks.

### PR #23+ — Carefully controlled production operations

- After separate security and operational sign-off, introduce authorized
  package updates, service management, configuration deployments and restarts.
- Require RBAC, approvals, explicit target scope, maintenance windows,
  audit trail and rollback plans before each category is enabled.

## Development and acceptance rules

1. Keep `main` stable. Work on feature branches with dedicated PRs.
2. No merge of a newly enabled remote execution mode before manual QA.
3. Do not include SSH private keys, passwords, tokens or session secrets in Git.
4. Each milestone needs positive and negative tests (rejections,
   permissions, cancellation, timeout and restart behavior).
5. Update this file after each accepted PR with the actual result and link.
6. Production access must be an explicit, separate approval decision.

## Next action

**Start PR #18: preflight and approval without execution.**
Do not connect repository playbooks to the remote executor until the
security isolation and approval gates are implemented.

## Longer-term milestones

The original release milestones below remain as a broader product
roadmap. They are aspirational and do not imply that optional SSO,
full container isolation or production automation have already shipped.

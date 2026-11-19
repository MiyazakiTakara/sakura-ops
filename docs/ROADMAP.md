# SakuraOps roadmap

SakuraOps is a self-hosted infrastructure management application, independent of Semaphore. Its primary interface is the `skr` administrative command language, available in the web console and, later, as a Linux CLI. A GUI will call the same backend actions.

## Current baseline: v0.1.0-dev
- Local admin bootstrap with Argon2 password hashes and SQLite.
- Local login and signed cookie sessions.
- Authenticated web console with `help`, `status`, `version`; command history and clear.
- Source on GitHub: `MiyazakiTakara/sakura-ops`; baseline commit `53d86f5`.
- This is a local development prototype, **not production-ready**. No real Ansible execution or SSO yet.

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

## Next work
Begin with **v0.1 security tests and hardening**, then implement optional SSO. Preserve the working prototype until these checks pass.

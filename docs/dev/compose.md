# Development Compose stack (NEXT-004)

`deployment/compose/compose.dev.yaml` (project `sakuraops-dev`) starts PostgreSQL, a one-shot
role bootstrap, a one-shot migrator and the hub for local development and CI. It is a lab and developer tool: not a production profile,
no TLS, no reverse proxy, no agent gateway.

## Prerequisites

- Docker Engine and the Compose plugin (v2.24 or newer for `--wait-timeout`), Linux.
  Membership in the `docker` group is root-equivalent on that host; use a disposable
  developer VM or WSL distribution.
- `make`, `bash`, `curl`.
- Port `127.0.0.1:8080` free, or set `SAKURAOPS_DEV_HUB_PORT=<port>` for both `make dev-up` and `make dev-smoke`
  (the binding stays on 127.0.0.1).

## Commands

```
make dev-secrets   # create the five files in .dev-secrets/ (random passwords, 0600) if missing
make dev-up        # build images; postgres -> db-bootstrap -> migrate -> hub, wait until healthy
make dev-smoke     # assert health, hardening, secret isolation and DB permissions of the running stack
make dev-scenarios # fresh start, restart, DB down, schema behind, migration failure (throwaway projects)
make dev-logs      # follow logs
make dev-down      # remove containers and networks, KEEP the database volume
make dev-clean CONFIRM=1   # also delete the database volume (refuses without CONFIRM=1)
```

Then open `http://127.0.0.1:8080/` (or `/spike/terminal`). `curl http://127.0.0.1:8080/healthz`
returns `{"status":"ok"}`.

## What is in the stack

Start order, enforced with `depends_on` conditions: `postgres` (`service_healthy`) ->
`db-bootstrap` (`service_completed_successfully`) -> `migrate` (`service_completed_successfully`)
-> `hub`. A failed bootstrap or migration therefore leaves the hub in state `created`.

| Service | Image | Notes |
| --- | --- | --- |
| `postgres` | `postgres:18-trixie` pinned by digest (PostgreSQL 18) | `POSTGRES_PASSWORD_FILE` from the admin secret, named volume `pgdata`, `pg_isready` health check, capabilities limited to those the image entrypoint needs, `no-new-privileges` |
| `db-bootstrap` | same pinned postgres image (ships `psql`) | runs `db/bootstrap/roles.sql` once per `up` with the admin credential; idempotent, resets the two role passwords; read-only rootfs, `cap_drop: ALL`, non-root |
| `migrate` | `hub.Dockerfile` target `migrator` (distroless, only `sakuraops-migrate`) | `sakuraops-migrate -database-url-file ... up` with the migrator DSN only; exits 0 when done; read-only rootfs, `cap_drop: ALL`, non-root |
| `hub` | `hub.Dockerfile` target `hub` | Node 22 builds the SPA, Go 1.27.4 builds a static `CGO_ENABLED=0 -trimpath` binary with the embedded UI, runtime is `distroless/static-debian13:nonroot` (no shell); started with `-database-url-file` for the app DSN only |

All base images are pinned by digest in the Dockerfile and compose file (the tag is in a
comment). Update them deliberately and review the diff; there is no automatic bump.

## Secrets and isolation

`make dev-secrets` writes, into `.dev-secrets/` (0700 directory, 0600 files, git-ignored,
excluded from the Docker build context, never printed):

| File | Holds | Mounted into |
| --- | --- | --- |
| `postgres_password` | admin (superuser `sakuraops_admin`) password | `postgres`, `db-bootstrap` |
| `migrator_password`, `app_password` | plain role passwords for the bootstrap | `db-bootstrap` |
| `migrator_database_url` | DSN of `sakuraops_migrator` | `migrate` |
| `app_database_url` | DSN of `sakuraops_app` | `hub` |

The DSN files are rewritten from the password files on every run. The hub therefore has no
path to the migrator or admin credential, and the migrator none to the admin one.

Outside Swarm, Compose bind-mounts file secrets with the host owner and mode and ignores
`uid`/`gid`/`mode`. The DSN reader (`internal/dsnfile`) refuses anything looser than 0640, so
the mode cannot be loosened to make a file readable. Instead `hub`, `migrate` and
`db-bootstrap` run as the invoking host user (`SAKURAOPS_DEV_UID`/`SAKURAOPS_DEV_GID`, filled
in by `make` from `id -u`/`id -g`; `make dev-up` refuses uid 0). They are still non-root, but
not uid 65532 as in the production image. `postgres` reads its password as root in the image
entrypoint, then drops to the `postgres` user. Running `docker compose` by hand needs those
two variables set. `SAKURAOPS_DEV_SECRETS_DIR` points the stack at another secrets directory
(used by `make dev-scenarios`).

## Database bootstrap assumptions

The bootstrap targets a **dedicated, clean database** created by this stack. `roles.sql`
rejects privileged and cross-role memberships of the two roles and re-applies their attributes,
but it does not audit existing ownership, ACLs, default privileges or other roles. Before
running it, or this wiring, against any database that already holds data or roles, audit
existing role memberships, object ownership and ACLs by hand. Do not use it against a shared or
non-lab database as is.

## Scenarios (`make dev-scenarios`)

Each scenario runs in throwaway projects (`sakuraops-scn`, `sakuraops-scn-bad`, override with
`SAKURAOPS_DEV_SCENARIO_PROJECT`) with their own generated secrets and volumes, all removed on
exit. Set `SAKURAOPS_DEV_HUB_PORT` if 8080 is taken.

1. **Fresh start**: empty volume; bootstrap and migrator exit 0, the baseline is applied,
   `/readyz` is 200 with `schema_version` equal to the latest migration.
2. **Restart on the existing volume**: `down` keeps the volume; on `up` the migrator reports
   "no migrations to apply", `/readyz` is 200 again, data written before survives.
3. **Database down**: `/readyz` is 503 `db_unavailable` with no connection details, `/healthz`
   stays 200 (liveness), and `/readyz` recovers after postgres is started again.
4. **Schema behind**: hub started alone after bootstrap only: `/readyz` is 503
   `schema_outdated`; after the migrator runs it is 200.
5. **Migration failure**: a wrong migrator credential makes `up` fail, `migrate` exits non-zero
   and logs `command failed` with a classification only, the hub is never started, nothing
   answers on the hub port, and no log contains a secret value or DSN.

Not covered at Compose level: a migration that fails inside SQL (needs a custom image); that is
covered by the integration tests (`make test-integration`).

## Security properties (checked by `make dev-smoke`)

- PostgreSQL has no `ports:` entry and is attached only to the `backend` network, which is
  `internal: true` (no route outside). The smoke test also tries an outbound connection
  from the postgres container and expects it to fail.
- The hub is published only on `127.0.0.1:8080`, through a separate `edge` network. It is
  attached to `backend` as well, to reach postgres.
- Hub container: non-root user, `read_only: true`, `cap_drop: [ALL]`,
  `no-new-privileges:true`, health check through the binary's own `-healthcheck` mode
  (`GET /healthz`, 3 s timeout, exit 0 or 1) because the image has no curl or shell.
- Each container holds only its own secret files (`docker cp` listing of `/run/secrets`), the
  hub DSN file is readable by the hub uid, and no secret value appears in `docker inspect` or
  container logs.
- Database permissions are exercised with real logins: the app role has no superuser, createdb,
  createrole, replication or bypassrls and no role memberships, cannot CREATE, ALTER, DROP,
  TRUNCATE, create temp tables, INSERT/DELETE settings, read or write the goose table, or
  `SET ROLE`; it can `SELECT` settings and call `sakuraops.schema_version()` (that function is
  the only way it learns the schema version, since it has no access to the goose table). The
  migrator can run DDL in its schema and read the goose table, but cannot create in `public`,
  roles or databases. The hub's database sessions all use the app role.

## Not here

- No TLS (the DSNs use `sslmode=disable` on the internal network; see "Transport security" in
  `docs/dev/database.md`), no HTTP/2 mTLS agent listener, no reverse proxy, no backups.
- Not for production, and not reachable from other machines by design. Change nothing here
  to expose it on a network without a separate review.

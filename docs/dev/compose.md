# Development Compose stack (NEXT-004)

`deployment/compose/compose.dev.yaml` (project `sakuraops-dev`) starts PostgreSQL and the
hub for local development and CI. It is a lab and developer tool: not a production profile,
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
make dev-secrets   # create .dev-secrets/postgres_password (random, 0600) if missing
make dev-up        # build the hub image, start postgres + hub, wait for both to be healthy
make dev-smoke     # assert health, isolation and hardening of the running stack
make dev-logs      # follow logs
make dev-down      # remove containers and networks, KEEP the database volume
make dev-clean CONFIRM=1   # also delete the database volume (refuses without CONFIRM=1)
```

Then open `http://127.0.0.1:8080/` (or `/spike/terminal`). `curl http://127.0.0.1:8080/healthz`
returns `{"status":"ok"}`.

## What is in the stack

| Service | Image | Notes |
| --- | --- | --- |
| `postgres` | `postgres:18-trixie` pinned by digest (PostgreSQL 18) | `POSTGRES_PASSWORD_FILE` from a Compose secret, named volume `pgdata`, `pg_isready` health check, capabilities limited to those the image entrypoint needs, `no-new-privileges` |
| `hub` | built from `deployment/docker/hub.Dockerfile` | Node 22 builds the SPA, Go 1.27.4 builds a static `CGO_ENABLED=0 -trimpath` binary with the embedded UI, runtime is `distroless/static-debian13:nonroot` (no shell), user 65532 |

All base images are pinned by digest in the Dockerfile and compose file (the tag is in a
comment). Update them deliberately and review the diff; there is no automatic bump.

## Security properties (checked by `make dev-smoke`)

- PostgreSQL has no `ports:` entry and is attached only to the `backend` network, which is
  `internal: true` (no route outside). The smoke test also tries an outbound connection
  from the postgres container and expects it to fail.
- The hub is published only on `127.0.0.1:8080`, through a separate `edge` network. It is
  attached to `backend` as well, so it can reach postgres once it uses the database.
- Hub container: non-root user, `read_only: true`, `cap_drop: [ALL]`,
  `no-new-privileges:true`, health check through the binary's own `-healthcheck` mode
  (`GET /healthz`, 3 s timeout, exit 0 or 1) because the image has no curl or shell.
- The database password is generated locally into `.dev-secrets/` (directory 0700, file 0600,
  git-ignored, excluded from the Docker build context) and is never printed.

## Not here

- The hub does not connect to the database yet. Migrations, roles and sqlc arrive in the
  database PR (`docs/dev/database.md`); wiring them into Compose follows after both merge.
- No TLS, no HTTP/2 mTLS agent listener, no reverse proxy, no backups.
- Not for production, and not reachable from other machines by design. Change nothing here
  to expose it on a network without a separate review.

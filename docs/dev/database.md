# Database: roles, migrations, queries (NEXT-004)

PostgreSQL 18 (image `postgres:18-trixie`, pinned by digest in the integration tests; the
same digest as the dev Compose file in `docs/dev/compose.md`, keep the two in step).
Nothing here connects to a real instance: tests start a throwaway container.

## Layout

| Path | Purpose |
| --- | --- |
| `db/bootstrap/roles.sql` | one-time, idempotent, run by a superuser: roles, schema, revokes |
| `db/migrations/NNNNN_name.sql` | goose migrations, embedded into the binaries (`db/migrations` package) |
| `db/queries/*.sql`, `sqlc.yaml` | sqlc queries; generated code is committed in `internal/store/dbgen` |
| `db/sqlc/prelude.sql` | sqlc-only stand-in for the schema and goose table created outside migrations |
| `cmd/sakuraops-migrate` | the migrator: `up`, `status`, `version` (no `down`) |
| `internal/store` | pgx pool for the hub, `SchemaVersion`, `GetInstanceSettings` |

## Roles

| Role | Can | Cannot |
| --- | --- | --- |
| `sakuraops_migrator` | owns schema `sakuraops` and everything in it, runs migrations, owns the goose table | create objects in `public`, superuser, create roles or databases |
| `sakuraops_app` (hub) | `USAGE` on schema, exactly the grants each migration gives it (baseline: `SELECT, UPDATE` on `instance_settings`), `EXECUTE sakuraops.schema_version()` | any DDL (`CREATE`, `ALTER`, `DROP`, `TRUNCATE`), temp tables, `INSERT`/`DELETE` on settings, read or write the goose table, `SET ROLE` |

- Everything lives in schema `sakuraops`; `CREATE` on `public` is revoked from `PUBLIC`, and
  `PUBLIC` has no `CONNECT` on the database. Both roles are `LOGIN` only (no superuser,
  createdb, createrole, replication, bypassrls).
- There are no default privileges: every migration must `GRANT` what the application needs.
- The hub learns the schema version through the `SECURITY DEFINER` function
  `sakuraops.schema_version()`, so it needs no access to the goose table. This is the one
  place where "app cannot touch the goose table" required an indirection.
- Run the bootstrap once as a superuser, passwords never hardcoded:

  ```
  psql -v ON_ERROR_STOP=1 -d <database> -f db/bootstrap/roles.sql
  ```

  with `SAKURAOPS_MIGRATOR_PASSWORD_FILE` and `SAKURAOPS_APP_PASSWORD_FILE` in the environment
  (files, preferred), or `-v migrator_password=... -v app_password=...` (visible in the process
  list; lab use only). Empty or missing passwords abort the script. Re-running resets the
  passwords and is otherwise a no-op.

## Migrator

```
sakuraops-migrate [-database-url-file PATH] [-timeout 2m] up|status|version
```

- The URL comes only from `-database-url-file` or `SAKURAOPS_DATABASE_URL_FILE` (a file path),
  never from argv content, and is never printed or logged. Driver errors are logged as a
  classification (`sqlstate`, `network error`, `timeout`) plus the server message for SQL
  errors, never host, user or URL.
- File policy (`internal/dsnfile`): refused unless the mode is 0600 or 0640 (any group write
  or any access for others is refused); group-readable 0640 is accepted with a warning;
  missing, non-regular, empty, or larger than 4 KiB is refused. Checks run on the opened
  descriptor.
- Concurrency: goose `Provider` with a PostgreSQL session advisory lock (polled every second,
  up to 2 minutes), so two migrators started together both exit 0 and each migration is
  applied once.
- Exit codes: `0` ok, `1` database or migration failure or timeout, `2` usage, `3` URL file
  or URL rejected.
- The goose table is `sakuraops.goose_db_version`.

## Policy

- **Forward-only.** Migrations have no `Down` section (a test rejects it) and the migrator has
  no `down` command. Rollback is restore from backup (docs/07-operations.md: backup before
  upgrade; roll back the image only if the schema stays compatible, otherwise restore).
- Migrations are immutable once released; fix forward with a new file.
- Destructive changes need a backup and restore plan in the PR (docs/22-data-model.md).
- Upgrade test: the integration tests apply an earlier version, write data through the
  application role, upgrade, and check the data survived.
  Only one real migration exists, so the second step is a test-only fixture migration in the
  same goose table; the mechanics are identical to a real `00002`. Add a real stepwise case
  with the first real follow-up migration.
- The hub requires `schema_version >= migrations.LatestVersion()` (compiled in) for `/readyz`.

## Hub readiness

`-database-url-file PATH` (optional) gives the hub a pool as `sakuraops_app`; the pool connects
lazily, so the hub starts while the database is down.

| `GET /readyz` | Meaning |
| --- | --- |
| 200 `{"status":"ready","schema_version":N}` | database reachable, schema version >= the compiled-in version |
| 503 `schema_outdated` | reachable, but not migrated or older than this binary |
| 503 `db_unavailable` | connection or query failed (details only in the redacted log) |
| 503 `db_not_configured` | started without `-database-url-file` |

Errors use the docs/23 envelope; no DSN or driver text is returned. `/readyz` is a reserved
path and never falls back to the SPA. `/healthz` stays a pure liveness probe.

## Commands

```
make sqlc               # regenerate internal/store/dbgen
make sqlc-check         # regenerate and fail on drift (CI)
make test-integration   # needs docker; starts and removes its own postgres container
go test -race ./internal/store/...
```

The integration suite starts a container on a random `127.0.0.1` port with random
passwords (env file mode 0600) and tmpfs data, runs the real `roles.sql`, and removes the
container afterwards. Cases: fresh migrate, re-run no-op, stepwise upgrade with data, two
concurrent migrator processes, in-process concurrent runners on a slow migration, application
role denials (DDL, goose table, singleton rows), role attributes, bootstrap idempotency and
password-file variant, singleton and check constraints, `/readyz` (unmigrated, current, newer
binary, container stopped), and the hub binary through its flag.

## Not included

- Wiring the hub and migrator into `deployment/compose/compose.dev.yaml` (follow-up after both
  PRs merge). Docker Compose file secrets are bind mounts that keep the host owner, so that
  follow-up must decide how the non-root hub reads its URL file.
- Backups and restore tooling, TLS to PostgreSQL (the test connections use `sslmode=disable`
  on loopback), connection poolers, row-level security.
- Any table other than `instance_settings`; the rest of docs/22-data-model.md arrives with
  NEXT-013, 014, 015, 018.

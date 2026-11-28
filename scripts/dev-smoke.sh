#!/usr/bin/env bash
# Smoke test for the running development stack (make dev-up first). Asserts health,
# network isolation, container hardening, per-container secret isolation and the database
# permissions of the app and migrator roles. Prints no secrets.
set -euo pipefail

export SAKURAOPS_DEV_UID="${SAKURAOPS_DEV_UID:-$(id -u)}" SAKURAOPS_DEV_GID="${SAKURAOPS_DEV_GID:-$(id -g)}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose=(docker compose -f "${root}/deployment/compose/compose.dev.yaml")
port="${SAKURAOPS_DEV_HUB_PORT:-8080}"
base="http://127.0.0.1:${port}"
fail=0

ok()  { echo "ok   - $1"; }
bad() { echo "FAIL - $1" >&2; fail=1; }
check() { # description, command...
  local d="$1"; shift
  if "$@" >/dev/null 2>&1; then ok "$d"; else bad "$d"; fi
}

hub="$("${compose[@]}" ps -q hub)"
pg="$("${compose[@]}" ps -q postgres)"
[[ -n "${hub}" && -n "${pg}" ]] || { echo "FAIL - stack is not running (make dev-up)" >&2; exit 1; }
mig="$("${compose[@]}" ps -a -q migrate)"
boot="$("${compose[@]}" ps -a -q db-bootstrap)"
[[ -n "${mig}" && -n "${boot}" ]] || { echo "FAIL - one-shot containers not found" >&2; exit 1; }
proj="$(docker inspect --format '{{index .Config.Labels "com.docker.compose.project"}}' "${hub}")"
secdir="${SAKURAOPS_DEV_SECRETS_DIR:-${root}/.dev-secrets}"

insp() { docker inspect --format "$1" "$2"; }

# Hub reachable on loopback and serving.
check "hub /healthz returns ok" bash -c "curl -fsS --max-time 5 ${base}/healthz | grep -q '\"status\":\"ok\"'"
check "hub /spike/terminal serves HTML" bash -c "curl -fsS --max-time 5 -D - -o /dev/null ${base}/spike/terminal | tr 'A-Z' 'a-z' | grep -q '^content-type: text/html'"
check "hub container health is healthy" test "$(insp '{{.State.Health.Status}}' "${hub}")" = healthy
check "postgres container health is healthy" test "$(insp '{{.State.Health.Status}}' "${pg}")" = healthy

# Postgres is not published to the host at all.
check "postgres has no published ports" test -z "$(insp '{{json .NetworkSettings.Ports}}' "${pg}" | grep -o '"HostPort"' || true)"
check "postgres attached to the backend network only" test "$(insp '{{range $k,$v := .NetworkSettings.Networks}}{{$k}} {{end}}' "${pg}")" = "${proj}_backend "
check "backend network is internal" test "$(docker network inspect --format '{{.Internal}}' ${proj}_backend)" = true
check "postgres container exec works (control for the next check)" docker exec "${pg}" bash -c 'timeout 3 bash -c "exec 3<>/dev/tcp/127.0.0.1/5432"'
check "postgres has no route outside the internal network" bash -c '! docker exec "$1" bash -c "timeout 3 bash -c \"exec 3<>/dev/tcp/1.1.1.1/53\""' _ "${pg}"

# Hub published only on loopback.
check "hub published only on 127.0.0.1:${port}" test "$(insp '{{range $p,$b := .NetworkSettings.Ports}}{{range $b}}{{.HostIp}}:{{.HostPort}} {{end}}{{end}}' "${hub}")" = "127.0.0.1:${port} "

# Hub hardening.
check "hub runs as non-root user" bash -c "u=\"\$(docker inspect --format '{{.Config.User}}' ${hub})\"; [[ -n \$u && \${u%%:*} != 0 && \${u%%:*} != root ]]"
hub_uids="$(docker top "${hub}" -eo pid,uid 2>/dev/null | awk 'NR>1 {print $2}' || true)"
check "hub process uid is not 0" bash -c '[[ -n "$1" ]] && ! grep -qx 0 <<<"$1"' _ "${hub_uids}"
check "hub rootfs is read-only" test "$(insp '{{.HostConfig.ReadonlyRootfs}}' "${hub}")" = true
check "hub drops all capabilities" test "$(insp '{{json .HostConfig.CapDrop}}' "${hub}")" = '["ALL"]'
check "hub has no-new-privileges" bash -c "docker inspect --format '{{json .HostConfig.SecurityOpt}}' ${hub} | grep -q no-new-privileges:true"

# PostgreSQL hardening: derived image without gosu, non-root, no capabilities at all.
want_user="${SAKURAOPS_DEV_UID}:${SAKURAOPS_DEV_GID}"
check "postgres container is configured as ${want_user} (not root)" test "$(insp '{{.Config.User}}' "${pg}")" = "${want_user}"
check "postgres process uid is not 0 and equals ${SAKURAOPS_DEV_UID}" test "$(docker exec "${pg}" id -u)" = "${SAKURAOPS_DEV_UID}"
pg_uids="$(docker top "${pg}" -eo pid,uid 2>/dev/null | awk 'NR>1 {print $2}' | sort -u | tr '\n' ' ' || true)"
check "every process in the postgres container runs as uid ${SAKURAOPS_DEV_UID}" test "${pg_uids}" = "${SAKURAOPS_DEV_UID} "
check "gosu is absent from the running postgres container" bash -c '! docker exec "$1" test -e /usr/local/bin/gosu && ! docker exec "$1" bash -c "command -v gosu"' _ "${pg}"
check "postgres has no added capabilities (CapAdd empty)" bash -c 'v="$(docker inspect --format "{{json .HostConfig.CapAdd}}" "$1")"; [[ "$v" == null || "$v" == "[]" ]]' _ "${pg}"
check "postgres drops all capabilities" test "$(insp '{{json .HostConfig.CapDrop}}' "${pg}")" = '["ALL"]'
check "postgres has no-new-privileges" bash -c "docker inspect --format '{{json .HostConfig.SecurityOpt}}' ${pg} | grep -q no-new-privileges:true"
check "postgres process holds no effective capability" bash -c 'docker exec "$1" grep -qx "CapEff:.0000000000000000" /proc/1/status' _ "${pg}"
check "postgres is not privileged" test "$(insp '{{.HostConfig.Privileged}}' "${pg}")" = false
check "PGDATA is owned by uid ${SAKURAOPS_DEV_UID} with mode 0700" bash -c '[[ "$(docker exec "$1" bash -c "stat -c \"%u %a\" \"\$PGDATA\"")" == "$2 700" ]]' _ "${pg}" "${SAKURAOPS_DEV_UID}"
check "postgres password file is owned by the postgres uid, mode 0600" bash -c 'docker cp "$1:/run/secrets/postgres_password" - | tar --numeric-owner -tvf - | grep -q "^-rw------- $2/"' _ "${pg}" "${SAKURAOPS_DEV_UID}"
check "postgres service uses the locally built derived image" test "$(insp '{{.Config.Image}}' "${pg}")" = sakuraops-dev-postgres:local
check "db-bootstrap uses the derived postgres image tag" test "$(insp '{{.Config.Image}}' "${boot}")" = sakuraops-dev-postgres:local

# --- Wiring: bootstrap -> migrate -> hub ---
latest="$(ls "${root}"/db/migrations/*.sql | sed 's#.*/0*\([0-9]*\)_.*#\1#' | sort -n | tail -1)"
exit_of() { docker inspect --format '{{.State.Status}} {{.State.ExitCode}}' "$1"; }
check "db-bootstrap exited 0" test "$(exit_of "${boot}")" = "exited 0"
check "migrate exited 0" test "$(exit_of "${mig}")" = "exited 0"
check "hub /readyz is 200 at the latest schema version (${latest})" bash -c "curl -fsS --max-time 5 ${base}/readyz | grep -q '\"schema_version\":${latest}[,}]'"

# --- Secret isolation: each container holds only its own secret files ---
secret_files() { docker cp "$1:/run/secrets" - 2>/dev/null | tar -t 2>/dev/null | grep -v '/$' | sed 's#.*/##' | sort | tr '\n' ' '; }
check "hub holds only app_database_url" test "$(secret_files "${hub}")" = "app_database_url "
check "migrate holds only migrator_database_url" test "$(secret_files "${mig}")" = "migrator_database_url "
check "db-bootstrap holds only admin and the two role passwords" test "$(secret_files "${boot}")" = "app_password migrator_password postgres_password "
check "postgres holds only the admin password" test "$(secret_files "${pg}")" = "postgres_password "
check "hub has no migrator or admin secret path" bash -c '! docker cp "$1:/run/secrets/migrator_database_url" - >/dev/null 2>&1 && ! docker cp "$1:/run/secrets/postgres_password" - >/dev/null 2>&1' _ "${hub}"
dsn_user() { docker cp "$1:/run/secrets/$2" - 2>/dev/null | tar -xO 2>/dev/null | sed -n 's#^postgres://\([^:]*\):.*#\1#p'; }
check "hub DSN names the app role" test "$(dsn_user "${hub}" app_database_url)" = sakuraops_app
check "migrate DSN names the migrator role" test "$(dsn_user "${mig}" migrator_database_url)" = sakuraops_migrator
# The hub user owns its file and can read it: the hub logs in with it (readyz above) and
# the owner and mode inside the container are the hub uid and 0600.
check "hub DSN file is owned by the hub uid, mode 0600" bash -c 'docker cp "$1:/run/secrets/app_database_url" - | tar --numeric-owner -tvf - | grep -q "^-rw------- $2/"' _ "${hub}" "$(docker inspect --format '{{.Config.User}}' "${hub}" | cut -d: -f1)"
# No secret value in inspect output (env, args) or in any container log.
leak=0
logs="$(for c in "${hub}" "${mig}" "${boot}" "${pg}"; do docker logs "$c" 2>&1; done)"
for f in postgres_password migrator_password app_password; do
  # grep -F -f takes the secret as the pattern; nothing is printed.
  if docker inspect "${hub}" "${mig}" "${boot}" "${pg}" | grep -qFf "${secdir}/${f}"; then leak=1; fi
  if grep -qFf "${secdir}/${f}" <<<"${logs}"; then leak=1; fi
done
check "no secret value in docker inspect or container logs" test "${leak}" = 0

# --- Database permissions, against the real running database ---
admin_sql() { docker exec -i "${pg}" psql -X -tA -v ON_ERROR_STOP=1 -U sakuraops_admin -d sakuraops -c "$1"; }
# Real login as a role: the password goes to the container through an env file on a pipe.
as_role() { # role password-file sql
  docker exec -i --env-file <(printf 'PGPASSWORD=%s\n' "$(cat "$2")") "${pg}" \
    psql -X -tA -v ON_ERROR_STOP=1 -h 127.0.0.1 -U "$1" -d sakuraops -c "$3"
}
denied() { # role password-file sql: must fail with a permission error
  local out
  if out="$(as_role "$1" "$2" "$3" 2>&1)"; then return 1; fi
  grep -qiE 'permission denied|must be owner' <<<"${out}"
}
allowed() { as_role "$1" "$2" "$3" >/dev/null 2>&1; }
app_pw="${secdir}/app_password"; mig_pw="${secdir}/migrator_password"

check "roles have no privileged attributes" test "$(admin_sql "SELECT count(*) FROM pg_roles WHERE rolname IN ('sakuraops_app','sakuraops_migrator') AND (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR NOT rolcanlogin)")" = 0
check "both roles exist" test "$(admin_sql "SELECT count(*) FROM pg_roles WHERE rolname IN ('sakuraops_app','sakuraops_migrator')")" = 2
check "roles are not members of any role" test "$(admin_sql "SELECT count(*) FROM pg_auth_members WHERE member IN (SELECT oid FROM pg_roles WHERE rolname IN ('sakuraops_app','sakuraops_migrator'))")" = 0
check "app role logs in with its DSN password and reads settings" allowed sakuraops_app "${app_pw}" "SELECT instance_name FROM sakuraops.instance_settings"
check "app role reads the schema version through the function" allowed sakuraops_app "${app_pw}" "SELECT sakuraops.schema_version()"
check "app role cannot CREATE in schema sakuraops" denied sakuraops_app "${app_pw}" "CREATE TABLE sakuraops.smoke_x (i int)"
check "app role cannot CREATE in public" denied sakuraops_app "${app_pw}" "CREATE TABLE public.smoke_x (i int)"
check "app role cannot ALTER a table" denied sakuraops_app "${app_pw}" "ALTER TABLE sakuraops.instance_settings ADD COLUMN smoke_x int"
check "app role cannot DROP a table" denied sakuraops_app "${app_pw}" "DROP TABLE sakuraops.instance_settings"
check "app role cannot TRUNCATE" denied sakuraops_app "${app_pw}" "TRUNCATE sakuraops.instance_settings"
check "app role cannot INSERT settings" denied sakuraops_app "${app_pw}" "INSERT INTO sakuraops.instance_settings (id, instance_name) VALUES (false, 'x')"
check "app role cannot DELETE settings" denied sakuraops_app "${app_pw}" "DELETE FROM sakuraops.instance_settings"
check "app role cannot read the goose table" denied sakuraops_app "${app_pw}" "SELECT * FROM sakuraops.goose_db_version"
check "app role cannot write the goose table" denied sakuraops_app "${app_pw}" "DELETE FROM sakuraops.goose_db_version"
check "app role cannot create temporary tables" denied sakuraops_app "${app_pw}" "CREATE TEMP TABLE smoke_x (i int)"
check "app role cannot SET ROLE to migrator" denied sakuraops_app "${app_pw}" "SET ROLE sakuraops_migrator"
check "app role cannot SET ROLE to admin" denied sakuraops_app "${app_pw}" "SET ROLE sakuraops_admin"
check "app role cannot CREATE ROLE" denied sakuraops_app "${app_pw}" "CREATE ROLE smoke_x"
check "app role cannot CREATE DATABASE" denied sakuraops_app "${app_pw}" "CREATE DATABASE smoke_x"
check "migrator role can DDL in its schema (transaction rolled back)" allowed sakuraops_migrator "${mig_pw}" "BEGIN; CREATE TABLE sakuraops.smoke_x (i int); DROP TABLE sakuraops.smoke_x; ROLLBACK"
check "migrator role can read the goose table" allowed sakuraops_migrator "${mig_pw}" "SELECT max(version_id) FROM sakuraops.goose_db_version"
check "migrator role cannot CREATE in public" denied sakuraops_migrator "${mig_pw}" "CREATE TABLE public.smoke_x (i int)"
check "migrator role cannot CREATE ROLE" denied sakuraops_migrator "${mig_pw}" "CREATE ROLE smoke_x"
check "migrator role cannot CREATE DATABASE" denied sakuraops_migrator "${mig_pw}" "CREATE DATABASE smoke_x"
check "hub holds no admin connection (only the app role)" test "$(admin_sql "SELECT string_agg(DISTINCT usename, ',') FROM pg_stat_activity WHERE application_name = 'sakuraops-hub'")" = sakuraops_app

exit "${fail}"

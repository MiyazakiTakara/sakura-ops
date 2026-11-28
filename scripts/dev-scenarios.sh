#!/usr/bin/env bash
# Scenario tests for the development stack wiring (postgres -> db-bootstrap -> migrate -> hub).
# Runs in throwaway Compose projects with their own generated secrets and volumes, never
# touching the developer's own stack, and removes everything on exit. Prints no secrets.
#   make dev-scenarios      (set SAKURAOPS_DEV_HUB_PORT if 8080 is taken)
set -euo pipefail

export SAKURAOPS_DEV_UID="${SAKURAOPS_DEV_UID:-$(id -u)}" SAKURAOPS_DEV_GID="${SAKURAOPS_DEV_GID:-$(id -g)}"
if [[ "${SAKURAOPS_DEV_UID}" == 0 ]]; then echo "refusing to run as uid 0" >&2; exit 1; fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
file="${root}/deployment/compose/compose.dev.yaml"
base="${SAKURAOPS_DEV_SCENARIO_PROJECT:-sakuraops-scn}"
port="${SAKURAOPS_DEV_HUB_PORT:-8080}"
url="http://127.0.0.1:${port}"
export SAKURAOPS_DEV_HUB_PORT="${port}"
latest="$(ls "${root}"/db/migrations/*.sql | sed 's#.*/0*\([0-9]*\)_.*#\1#' | sort -n | tail -1)"

work="$(mktemp -d)"
good="${work}/good"; bad="${work}/bad"
fail=0
proj="${base}"

dc() { docker compose -p "${proj}" -f "${file}" "$@"; }
cleanup() {
  for p in "${base}" "${base}-bad"; do
    SAKURAOPS_DEV_SECRETS_DIR="${good}" docker compose -p "${p}" -f "${file}" down --volumes --remove-orphans >/dev/null 2>&1 || true
  done
  rm -rf "${work}"
}
trap cleanup EXIT

ok()  { echo "ok   - $1"; }
bad() { echo "FAIL - $1" >&2; fail=1; }
check() { local d="$1"; shift; if "$@" >/dev/null 2>&1; then ok "$d"; else bad "$d"; fi; }

# code_of PATH -> HTTP status ("000" when nothing answers)
code_of() { curl -s -o "${work}/body" -w '%{http_code}' --max-time 5 "${url}$1" || true; }
# wait_code PATH CODE SECONDS: poll until the status matches; body kept in ${work}/body
wait_code() {
  local end=$((SECONDS + $3))
  while ((SECONDS < end)); do
    [[ "$(code_of "$1")" == "$2" ]] && return 0
    sleep 2
  done
  return 1
}
body_has() { grep -q "$1" "${work}/body"; }
cid() { dc ps -a -q "$1"; }
exit_state() { docker inspect --format '{{.State.Status}} {{.State.ExitCode}}' "$(cid "$1")"; }
logs_of() { dc logs --no-color "$@" 2>&1; }
admin_sql() { dc exec -T postgres psql -X -tA -v ON_ERROR_STOP=1 -U sakuraops_admin -d sakuraops -c "$1"; }
no_secret_in() { # text-file ...: none of the generated secret values appears
  local f
  for f in postgres_password migrator_password app_password; do
    if grep -qFf "${good}/${f}" "$@"; then return 1; fi
  done
}

# Fails at once, stopping nothing, when something already listens on the hub port (for
# example a dev stack left running); otherwise that stack would answer our probes.
require_port_free() {
  if (exec 3<>"/dev/tcp/127.0.0.1/${port}") 2>/dev/null; then
    echo "FAIL - 127.0.0.1:${port} is already in use (another stack running?). Stop it or set SAKURAOPS_DEV_HUB_PORT to a free port; nothing was changed." >&2
    exit 1
  fi
}
require_port_free

SAKURAOPS_DEV_SECRETS_DIR="${good}" "${root}/scripts/dev-secrets.sh" >/dev/null
export SAKURAOPS_DEV_SECRETS_DIR="${good}"

dc build >/dev/null 2>&1 || { echo "FAIL - image build failed" >&2; exit 1; }
echo "== scenario 1: fresh start (empty volume)"
require_port_free
dc up -d --build --wait --wait-timeout 300 >/dev/null 2>&1 || bad "fresh: dev stack did not become healthy"
check "fresh: db-bootstrap and migrate exited 0" bash -c '[[ "$1" == "exited 0" && "$2" == "exited 0" ]]' _ "$(exit_state db-bootstrap)" "$(exit_state migrate)"
check "fresh: migrate applied the baseline" bash -c 'grep -q "applied 00001" <<<"$1"' _ "$(logs_of migrate)"
check "fresh: /readyz is 200 at schema version ${latest}" bash -c "[[ \$(curl -s --max-time 5 -o /dev/null -w '%{http_code}' ${url}/readyz) == 200 ]]"
wait_code /readyz 200 30 && body_has "\"schema_version\":${latest}" && ok "fresh: /readyz body reports schema_version ${latest}" || bad "fresh: /readyz body"
admin_sql "UPDATE sakuraops.instance_settings SET instance_name = 'scenario-marker'" >/dev/null || bad "fresh: could not write marker row"

echo "== scenario 2: restart with the existing volume"
dc down --remove-orphans >/dev/null 2>&1
require_port_free
dc up -d --wait --wait-timeout 300 >/dev/null 2>&1 || bad "restart: dev stack did not become healthy"
check "restart: migrate is a no-op" bash -c 'grep -q "no migrations to apply" <<<"$1"' _ "$(logs_of migrate)"
check "restart: bootstrap and migrate exited 0" bash -c '[[ "$1" == "exited 0" && "$2" == "exited 0" ]]' _ "$(exit_state db-bootstrap)" "$(exit_state migrate)"
wait_code /readyz 200 30 && ok "restart: /readyz is 200 again" || bad "restart: /readyz is not 200"
check "restart: data survived (marker row)" bash -c '[[ "$1" == scenario-marker ]]' _ "$(admin_sql "SELECT instance_name FROM sakuraops.instance_settings")"

echo "== scenario 3: database down, then back"
dc stop postgres >/dev/null 2>&1
wait_code /readyz 503 40 && body_has '"db_unavailable"' && ok "db down: /readyz is 503 db_unavailable" || bad "db down: expected 503 db_unavailable"
check "db down: response carries no connection details" bash -c '! grep -qiE "postgres://|password|sakuraops_" "$1"' _ "${work}/body"
check "db down: /healthz (liveness) stays 200" bash -c "[[ \$(curl -s --max-time 5 -o /dev/null -w '%{http_code}' ${url}/healthz) == 200 ]]"
dc start postgres >/dev/null 2>&1
wait_code /readyz 200 90 && ok "db back: /readyz is 200 again" || bad "db back: /readyz did not recover"

echo "== scenario 4: schema behind the binary"
dc down --volumes --remove-orphans >/dev/null 2>&1
dc up -d --wait --wait-timeout 300 postgres >/dev/null 2>&1 || bad "behind: postgres did not become healthy"
dc run --rm --no-deps db-bootstrap >/dev/null 2>&1 || bad "behind: bootstrap failed"
require_port_free
dc up -d --no-deps --wait --wait-timeout 120 hub >/dev/null 2>&1 || bad "behind: hub did not start"
wait_code /readyz 503 30 && body_has '"schema_outdated"' && ok "behind: /readyz is 503 schema_outdated" || bad "behind: expected 503 schema_outdated"
dc run --rm --no-deps migrate >/dev/null 2>&1 || bad "behind: migrate failed"
wait_code /readyz 200 30 && ok "behind: /readyz is 200 after the migrator ran" || bad "behind: /readyz did not become 200"
dc down --volumes --remove-orphans >/dev/null 2>&1

echo "== scenario 5: migration failure (wrong migrator credential) keeps the hub down"
mkdir -p "${bad}"; chmod 0700 "${bad}"
cp -p "${good}"/* "${bad}/"
sed -i 's#:[0-9a-f]\{64\}@#:wrong-credential@#' "${bad}/migrator_database_url"
proj="${base}-bad"
export SAKURAOPS_DEV_SECRETS_DIR="${bad}"
require_port_free
if dc up -d --wait --wait-timeout 300 >/dev/null 2>&1; then bad "failure: up succeeded with a bad migrator credential"; else ok "failure: dev stack up fails"; fi
check "failure: migrate exited non-zero" bash -c '[[ "$1" =~ ^exited\ [1-9] ]]' _ "$(exit_state migrate)"
check "failure: hub never started" bash -c '[[ "$1" == "created" ]]' _ "$(docker inspect --format '{{.State.Status}}' "$(cid hub)")"
check "failure: nothing answers /readyz on the hub port" bash -c "[[ \$(curl -s --max-time 3 -o /dev/null -w '%{http_code}' ${url}/readyz) == 000 ]]"
logs_of migrate >"${work}/migrate.log"
check "failure: the migrator reports the failure" grep -q 'command failed' "${work}/migrate.log"
logs_of >"${work}/all.log"
check "failure: logs contain no secret and no DSN" bash -c '! grep -q "postgres://" "$1" && ! grep -q "wrong-credential" "$1"' _ "${work}/all.log"
export SAKURAOPS_DEV_SECRETS_DIR="${good}"
check "failure: logs contain no generated secret value" no_secret_in "${work}/all.log"
dc down --volumes --remove-orphans >/dev/null 2>&1

exit "${fail}"

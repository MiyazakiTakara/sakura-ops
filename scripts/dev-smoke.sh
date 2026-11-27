#!/usr/bin/env bash
# Smoke test for the running development stack (make dev-up first). Asserts health,
# network isolation and container hardening. Prints no secrets.
set -euo pipefail

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

insp() { docker inspect --format "$1" "$2"; }

# Hub reachable on loopback and serving.
check "hub /healthz returns ok" bash -c "curl -fsS --max-time 5 ${base}/healthz | grep -q '\"status\":\"ok\"'"
check "hub /spike/terminal serves HTML" bash -c "curl -fsS --max-time 5 -D - -o /dev/null ${base}/spike/terminal | tr 'A-Z' 'a-z' | grep -q '^content-type: text/html'"
check "hub container health is healthy" test "$(insp '{{.State.Health.Status}}' "${hub}")" = healthy
check "postgres container health is healthy" test "$(insp '{{.State.Health.Status}}' "${pg}")" = healthy

# Postgres is not published to the host at all.
check "postgres has no published ports" test -z "$(insp '{{json .NetworkSettings.Ports}}' "${pg}" | grep -o '"HostPort"' || true)"
check "postgres attached to the backend network only" test "$(insp '{{range $k,$v := .NetworkSettings.Networks}}{{$k}} {{end}}' "${pg}")" = "sakuraops-dev_backend "
check "backend network is internal" test "$(docker network inspect --format '{{.Internal}}' sakuraops-dev_backend)" = true
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

exit "${fail}"

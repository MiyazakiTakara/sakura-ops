#!/usr/bin/env bash
# Creates the generated development secrets in .dev-secrets/ (git-ignored, 0700 dir,
# 0600 files) if they do not exist yet. Never prints a secret value.
#   postgres_password        admin (superuser) password: postgres and db-bootstrap
#   migrator_password        sakuraops_migrator password: db-bootstrap
#   app_password             sakuraops_app password: db-bootstrap
#   migrator_database_url    DSN of sakuraops_migrator: the migrate job only
#   app_database_url         DSN of sakuraops_app: the hub only
# The two DSN files are rewritten from the password files on every run, so they cannot
# drift from them. Passwords are hex: no characters that need URL quoting.
set -euo pipefail
umask 077

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dir="${SAKURAOPS_DEV_SECRETS_DIR:-${root}/.dev-secrets}"

mkdir -p "${dir}"
chmod 0700 "${dir}"

ensure_password() { # name
  local file="${dir}/$1"
  if [[ -s "${file}" ]]; then
    chmod 0600 "${file}"
    echo "dev-secrets: $1 already present (kept)"
    return
  fi
  head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' >"${file}"
  chmod 0600 "${file}"
  echo "dev-secrets: created $1 (value not shown)"
}

write_dsn() { # name role password-file
  local file="${dir}/$1" pw
  pw="$(cat "${dir}/$3")"
  printf 'postgres://%s:%s@postgres:5432/sakuraops?sslmode=disable\n' "$2" "${pw}" >"${file}"
  chmod 0600 "${file}"
}

ensure_password postgres_password
ensure_password migrator_password
ensure_password app_password
write_dsn migrator_database_url sakuraops_migrator migrator_password
write_dsn app_database_url sakuraops_app app_password
echo "dev-secrets: database URL files written (values not shown)"

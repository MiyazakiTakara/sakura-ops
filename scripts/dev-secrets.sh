#!/usr/bin/env bash
# Creates the generated development secrets in .dev-secrets/ (git-ignored, 0700 dir,
# 0600 files) if they do not exist yet. Never prints a secret value.
set -euo pipefail
umask 077

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dir="${root}/.dev-secrets"
file="${dir}/postgres_password"

mkdir -p "${dir}"
chmod 0700 "${dir}"

if [[ -s "${file}" ]]; then
  chmod 0600 "${file}"
  echo "dev-secrets: postgres_password already present (kept)"
  exit 0
fi

# 32 random bytes as hex: no characters that need quoting anywhere.
head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' >"${file}"
chmod 0600 "${file}"
echo "dev-secrets: created postgres_password (value not shown)"

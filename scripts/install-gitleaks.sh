#!/usr/bin/env bash
# Installs a pinned gitleaks release into bin/tools/ after verifying a pinned
# SHA-256 of the release archive. Linux x86_64 only (the P01 CI and dev target).
set -euo pipefail

VERSION="8.30.1"
ARCHIVE="gitleaks_${VERSION}_linux_x64.tar.gz"
SHA256="551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb"
URL="https://github.com/gitleaks/gitleaks/releases/download/v${VERSION}/${ARCHIVE}"

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dest="${root}/bin/tools"
bin="${dest}/gitleaks"

if [[ "$(uname -s)-$(uname -m)" != "Linux-x86_64" ]]; then
  echo "install-gitleaks: only Linux x86_64 is supported by this script" >&2
  exit 1
fi

if [[ -x "${bin}" ]] && "${bin}" version 2>/dev/null | grep -qx "${VERSION}"; then
  echo "gitleaks ${VERSION} already installed at ${bin}"
  exit 0
fi

tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

curl --proto '=https' --tlsv1.2 -fsSL -o "${tmp}/${ARCHIVE}" "${URL}"
echo "${SHA256}  ${tmp}/${ARCHIVE}" | sha256sum --check --status \
  || { echo "install-gitleaks: SHA-256 mismatch for ${ARCHIVE}" >&2; exit 1; }

mkdir -p "${dest}"
tar -xzf "${tmp}/${ARCHIVE}" -C "${tmp}" gitleaks
install -m 0755 "${tmp}/gitleaks" "${bin}"
echo "installed gitleaks ${VERSION} to ${bin}"

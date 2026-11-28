#!/usr/bin/env bash
# Installs a pinned grype release into bin/tools/ after verifying a pinned
# SHA-256 of the release archive. Linux x86_64 only (the P01 CI and dev target).
set -euo pipefail

VERSION="0.122.1"
ARCHIVE="grype_${VERSION}_linux_amd64.tar.gz"
SHA256="f1ba7e2917ee1395618de20a98da0aa7c0fb1788927797651047c50502a37538"
URL="https://github.com/anchore/grype/releases/download/v${VERSION}/${ARCHIVE}"

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dest="${root}/bin/tools"
bin="${dest}/grype"

if [[ "$(uname -s)-$(uname -m)" != "Linux-x86_64" ]]; then
  echo "install-grype: only Linux x86_64 is supported by this script" >&2
  exit 1
fi

if [[ -x "${bin}" ]] && "${bin}" --version 2>/dev/null | grep -q " ${VERSION}$"; then
  echo "grype ${VERSION} already installed at ${bin}"
  exit 0
fi

tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

curl --proto '=https' --tlsv1.2 -fsSL -o "${tmp}/${ARCHIVE}" "${URL}"
echo "${SHA256}  ${tmp}/${ARCHIVE}" | sha256sum --check --status \
  || { echo "install-grype: SHA-256 mismatch for ${ARCHIVE}" >&2; exit 1; }

mkdir -p "${dest}"
tar -xzf "${tmp}/${ARCHIVE}" -C "${tmp}" grype
install -m 0755 "${tmp}/grype" "${bin}"
echo "installed grype ${VERSION} to ${bin}"

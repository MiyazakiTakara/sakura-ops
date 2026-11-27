#!/usr/bin/env bash
# Installs a pinned syft release into bin/tools/ after verifying a pinned
# SHA-256 of the release archive. Linux x86_64 only (the P01 CI and dev target).
set -euo pipefail

VERSION="1.57.2"
ARCHIVE="syft_${VERSION}_linux_amd64.tar.gz"
SHA256="ad92bd3ec1b94a3df01d4d0aec199afa9c42b72e86da436ff5f837914d32706b"
URL="https://github.com/anchore/syft/releases/download/v${VERSION}/${ARCHIVE}"

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dest="${root}/bin/tools"
bin="${dest}/syft"

if [[ "$(uname -s)-$(uname -m)" != "Linux-x86_64" ]]; then
  echo "install-syft: only Linux x86_64 is supported by this script" >&2
  exit 1
fi

if [[ -x "${bin}" ]] && "${bin}" --version 2>/dev/null | grep -q " ${VERSION}$"; then
  echo "syft ${VERSION} already installed at ${bin}"
  exit 0
fi

tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

curl --proto '=https' --tlsv1.2 -fsSL -o "${tmp}/${ARCHIVE}" "${URL}"
echo "${SHA256}  ${tmp}/${ARCHIVE}" | sha256sum --check --status \
  || { echo "install-syft: SHA-256 mismatch for ${ARCHIVE}" >&2; exit 1; }

mkdir -p "${dest}"
tar -xzf "${tmp}/${ARCHIVE}" -C "${tmp}" syft
install -m 0755 "${tmp}/syft" "${bin}"
echo "installed syft ${VERSION} to ${bin}"

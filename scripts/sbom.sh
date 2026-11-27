#!/usr/bin/env bash
# Generates CycloneDX JSON SBOMs into bin/sbom/ with the pinned syft:
#   go.cdx.json             Go main module (go.mod, licences from the module cache)
#   npm-prod.cdx.json       npm production tree, installed from the lockfile with --omit=dev
#   npm-dev.cdx.json        npm production plus dev tree, installed from the lockfile
#   server-binary.cdx.json  the built bin/sakuraops-server (run `make build` first)
# npm installs run in a scratch copy of package.json and package-lock.json (scripts
# disabled) so the real frontend/node_modules is untouched. SBOMs hold no secrets.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${root}"
syft="${root}/bin/tools/syft"
out="${root}/bin/sbom"
work="${root}/bin/sbom-work"

[[ -x "${syft}" ]] || { echo "sbom: run scripts/install-syft.sh first" >&2; exit 1; }
[[ -f bin/sakuraops-server ]] || { echo "sbom: bin/sakuraops-server missing; run 'make build'" >&2; exit 1; }

export SYFT_CHECK_FOR_APP_UPDATE=false
# Go modules carry no licence metadata in go.mod; syft reads LICENSE files from the module cache.
export SYFT_GOLANG_SEARCH_LOCAL_MOD_CACHE_LICENSES=true

rm -rf "${out}" "${work}"
mkdir -p "${out}" "${work}"
trap 'rm -rf "${work}"' EXIT

go mod download

"${syft}" scan "dir:${root}" -q \
  --override-default-catalogers go-module-file-cataloger \
  --exclude './tools/**' --exclude './frontend/**' --exclude './bin/**' \
  --source-name sakuraops --source-version dev \
  -o "cyclonedx-json=${out}/go.cdx.json"

for kind in prod dev; do
  dir="${work}/${kind}"
  mkdir -p "${dir}"
  cp frontend/package.json frontend/package-lock.json "${dir}/"
  flags=(--ignore-scripts --no-audit --no-fund)
  [[ "${kind}" == "prod" ]] && flags+=(--omit=dev)
  (cd "${dir}" && npm ci "${flags[@]}" >/dev/null)
  "${syft}" scan "dir:${dir}" -q \
    --override-default-catalogers javascript-package-cataloger \
    -o "cyclonedx-json=${out}/npm-${kind}.cdx.json"
done

"${syft}" scan "file:${root}/bin/sakuraops-server" -q \
  --override-default-catalogers go-module-binary-cataloger \
  -o "cyclonedx-json=${out}/server-binary.cdx.json"

ls -l "${out}"

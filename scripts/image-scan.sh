#!/usr/bin/env bash
# Builds the hub image locally (same Dockerfile as Compose) and scans it plus the
# digest-pinned PostgreSQL image from compose.dev.yaml with the pinned grype.
# Reports (grype JSON, CycloneDX SBOMs) go to bin/image-scan/; they hold package
# names and versions only. Policy is applied by cmd/imagescan, not by grype.
# Env: IMAGE_TAG (default sakuraops-hub:image-scan-local), GO (default go).
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${root}"
grype="${root}/bin/tools/grype"
syft="${root}/bin/tools/syft"
out="${root}/bin/image-scan"
tag="${IMAGE_TAG:-sakuraops-hub:image-scan-local}"
compose="deployment/compose/compose.dev.yaml"

[[ -x "${grype}" ]] || { echo "image-scan: run scripts/install-grype.sh first" >&2; exit 1; }
[[ -x "${syft}" ]] || { echo "image-scan: run scripts/install-syft.sh first" >&2; exit 1; }

pg="$(sed -n 's/^ *image: *\(postgres@sha256:[0-9a-f]\{64\}\) *$/\1/p' "${compose}")"
[[ "$(printf '%s\n' "${pg}" | grep -c .)" == 1 ]] \
  || { echo "image-scan: expected exactly one digest-pinned postgres image in ${compose}" >&2; exit 1; }

rm -rf "${out}"
mkdir -p "${out}"

docker build -q -f deployment/docker/hub.Dockerfile -t "${tag}" . >/dev/null
docker pull -q "${pg}" >/dev/null

export GRYPE_CHECK_FOR_APP_UPDATE=false SYFT_CHECK_FOR_APP_UPDATE=false
"${grype}" db update >/dev/null

scan() { # label image
  "${grype}" "docker:$2" -q -o json --file "${out}/$1.grype.json"
  "${syft}" scan "docker:$2" -q -o "cyclonedx-json=${out}/$1.cdx.json"
}
scan hub "${tag}"
scan postgres "${pg}"

# Output is the policy verdict; exit status 1 means findings.
"${GO:-go}" run ./cmd/imagescan -ignore supply-chain/image-scan-ignore.json \
  "hub=${out}/hub.grype.json" "postgres=${out}/postgres.grype.json"

#!/usr/bin/env bash
# Scans every image the dev Compose file uses with the pinned grype:
#  - external images (digest-pinned in compose.dev.yaml, deduplicated) are pulled by digest;
#  - every distinct build: section is built from its own allowlisted Dockerfile
#    (docker build -f <dockerfile> --target; no target = last stage; build args at
#    their Dockerfile defaults) and the FINAL image is scanned.
# The image set comes from `imagescan plan` (YAML parsing in Go), not from grep.
# Every image is scanned in two modes, both mandatory and united by cmd/imagescan:
#   direct  grype docker:<image>
#   sbom    syft docker:<image> -o syft-json, then grype sbom:<that file>
# (grype 0.122.1 misses some binary-embedded Go stdlib packages in direct image
# scans that it finds from a syft SBOM; see docs/dev/supply-chain.md.) Each report
# and the syft SBOM are checked against the expected digest or image ID, tool
# versions and shape by cmd/imagescan before the policy is applied.
# Output in bin/image-scan/ per label: <label>.grype.json (direct),
# <label>.sbom.grype.json (sbom mode), <label>.syft.json (the SBOM scanned) and
# <label>.cdx.json (CycloneDX); package names and versions only. Needs docker and network.
# Env: GO (default go), IMAGE_SCAN_COMPOSE (default deployment/compose/compose.dev.yaml).
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${root}"
grype="${root}/bin/tools/grype"
syft="${root}/bin/tools/syft"
out="${root}/bin/image-scan"
compose="${IMAGE_SCAN_COMPOSE:-deployment/compose/compose.dev.yaml}"
go="${GO:-go}"

[[ -x "${grype}" ]] || { echo "image-scan: run scripts/install-grype.sh first" >&2; exit 1; }
[[ -x "${syft}" ]] || { echo "image-scan: run scripts/install-syft.sh first" >&2; exit 1; }
grype_version="$(sed -n 's/^VERSION="\([0-9.]*\)"$/\1/p' scripts/install-grype.sh)"
[[ -n "${grype_version}" ]] || { echo "image-scan: cannot read pinned grype version" >&2; exit 1; }
syft_version="$(sed -n 's/^VERSION="\([0-9.]*\)"$/\1/p' scripts/install-syft.sh)"
[[ -n "${syft_version}" ]] || { echo "image-scan: cannot read pinned syft version" >&2; exit 1; }

plan="$("${go}" run ./cmd/imagescan plan -compose "${compose}")"
[[ -n "${plan}" ]] || { echo "image-scan: empty image plan for ${compose}" >&2; exit 1; }

rm -rf "${out}"
mkdir -p "${out}"
export GRYPE_CHECK_FOR_APP_UPDATE=false SYFT_CHECK_FOR_APP_UPDATE=false
"${grype}" db update >/dev/null

scan() { # label image
  "${grype}" "docker:$2" -q -o json --file "${out}/$1.grype.json"
  "${syft}" scan "docker:$2" -q -o "cyclonedx-json=${out}/$1.cdx.json" -o "syft-json=${out}/$1.syft.json"
  "${grype}" "sbom:${out}/$1.syft.json" -q -o json --file "${out}/$1.sbom.grype.json"
}
inputs() { echo "$1=${out}/$1.grype.json,sbom-report=${out}/$1.sbom.grype.json,syft=${out}/$1.syft.json"; }

args=()
while IFS=$'\t' read -r kind label a b _; do
  case "${kind}" in
    external) # a=repo@digest b=digest
      docker pull -q "${a}" >/dev/null
      scan "${label}" "${a}"
      args+=("$(inputs "${label}"),digest=${b}")
      ;;
    build) # a=dockerfile (allowlisted by the plan) b=target or "-" (fourth field, the service, is unused)
      tag="sakuraops-image-scan-${label}:local"
      bargs=(-q -f "${a}" -t "${tag}")
      [[ "${b}" == "-" ]] || bargs+=(--target "${b}")
      docker build "${bargs[@]}" . >/dev/null
      id="$(docker image inspect --format '{{.Id}}' "${tag}")"
      [[ "${id}" =~ ^sha256:[0-9a-f]{64}$ ]] || { echo "image-scan: unexpected image ID for ${tag}" >&2; exit 1; }
      scan "${label}" "${tag}"
      args+=("$(inputs "${label}"),id=${id}")
      ;;
    *) echo "image-scan: unknown plan line kind ${kind}" >&2; exit 1 ;;
  esac
done <<<"${plan}"

# Output is the policy verdict; exit status 1 means findings.
"${go}" run ./cmd/imagescan -ignore supply-chain/image-scan-ignore.json \
  -grype-version "${grype_version}" -syft-version "${syft_version}" "${args[@]}"

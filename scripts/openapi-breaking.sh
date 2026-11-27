#!/bin/sh
# openapi-breaking.sh BASE_REF REPO_ROOT SPEC_PATH
#
# Runs `oasdiff breaking` for REPO_ROOT/SPEC_PATH against the same file at BASE_REF.
# Fails closed, like proto-breaking.sh:
#   - BASE_REF that does not resolve to a commit        -> error, exit 1
#   - any git error while inspecting the base commit     -> error, exit 1
#   - verified base commit without SPEC_PATH             -> skip (new contract), exit 0
#   - otherwise                                          -> oasdiff breaking, fails on ERR or WARN
# OASDIFF is the oasdiff command (default "oasdiff"); it is word-split on purpose.
set -eu

if [ "$#" -ne 3 ]; then
	echo "usage: openapi-breaking.sh BASE_REF REPO_ROOT SPEC_PATH" >&2
	exit 2
fi
base=$1
root=$2
spec=$3
OASDIFF=${OASDIFF:-oasdiff}

if ! sha=$(git -C "$root" rev-parse --verify --quiet "${base}^{commit}"); then
	echo "openapi-breaking: ERROR: base ref '$base' does not resolve to a commit (missing ref, shallow clone or git error); refusing to skip" >&2
	exit 1
fi
if [ -z "$sha" ]; then
	echo "openapi-breaking: ERROR: base ref '$base' resolved to an empty value" >&2
	exit 1
fi

if ! listing=$(git -C "$root" ls-tree --name-only "$sha" -- "$spec"); then
	echo "openapi-breaking: ERROR: git ls-tree failed for $sha (base '$base')" >&2
	exit 1
fi
if [ -z "$listing" ]; then
	echo "openapi-breaking: base $base ($sha) has no $spec; new contract, nothing to compare"
	exit 0
fi
if [ ! -f "$root/$spec" ]; then
	echo "openapi-breaking: ERROR: $spec exists on base $sha but is missing in the working tree (deleted contract)" >&2
	exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM
if ! git -C "$root" show "$sha:$spec" >"$tmp/base.yaml"; then
	echo "openapi-breaking: ERROR: git show $sha:$spec failed" >&2
	exit 1
fi

echo "openapi-breaking: comparing $spec against $base ($sha)"
# shellcheck disable=SC2086
$OASDIFF breaking --fail-on WARN "$tmp/base.yaml" "$root/$spec"

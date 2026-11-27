#!/bin/sh
# proto-breaking.sh BASE_REF REPO_ROOT
#
# Runs `buf breaking` (FILE) for REPO_ROOT/proto against BASE_REF. Fails closed:
#   - BASE_REF that does not resolve to a commit        -> error, exit 1
#   - any git error while inspecting the base commit     -> error, exit 1
#   - verified base commit without proto/buf.yaml        -> skip (new contract), exit 0
#   - otherwise                                          -> buf breaking against the commit SHA
# BUF is the buf command (default "buf"); it is word-split on purpose, e.g. "go tool ... buf".
set -eu

if [ "$#" -ne 2 ]; then
	echo "usage: proto-breaking.sh BASE_REF REPO_ROOT" >&2
	exit 2
fi
base=$1
root=$2
BUF=${BUF:-buf}

if ! sha=$(git -C "$root" rev-parse --verify --quiet "${base}^{commit}"); then
	echo "proto-breaking: ERROR: base ref '$base' does not resolve to a commit (missing ref, shallow clone or git error); refusing to skip" >&2
	exit 1
fi
if [ -z "$sha" ]; then
	echo "proto-breaking: ERROR: base ref '$base' resolved to an empty value" >&2
	exit 1
fi

if ! listing=$(git -C "$root" ls-tree --name-only "$sha" -- proto/buf.yaml); then
	echo "proto-breaking: ERROR: git ls-tree failed for $sha (base '$base')" >&2
	exit 1
fi
if [ -z "$listing" ]; then
	echo "proto-breaking: base $base ($sha) has no proto/ module; new contract, nothing to compare"
	exit 0
fi

cd "$root/proto"
# shellcheck disable=SC2086
exec $BUF breaking --against "$root/.git#ref=$sha,subdir=proto"

#!/bin/sh
# Tests scripts/openapi-breaking.sh in throwaway git repos. No network.
# Env: OASDIFF (oasdiff command, default "oasdiff"). Run from the repo root: make openapi-breaking-test
set -eu

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/.." && pwd)
script=$here/openapi-breaking.sh
OASDIFF=${OASDIFF:-oasdiff}
export OASDIFF
spec=api/openapi/sakuraops.v1.yaml
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM
fails=0

gitq() { git -c user.name=t -c user.email=t@example.invalid -c commit.gpgsign=false "$@"; }
ok()   { echo "PASS: $1"; }
bad()  { echo "FAIL: $1"; fails=$((fails + 1)); }

newrepo() {
	mkdir -p "$1"
	gitq init -q "$1"
	echo x >"$1/README"
	gitq -C "$1" add -A
	gitq -C "$1" commit -q -m base
}
addspec() {
	mkdir -p "$1/api/openapi"
	cp "$repo/$spec" "$1/$spec"
	# `go tool -modfile` needs a main module around the working directory.
	[ -f "$1/go.mod" ] || printf 'module probe\n\ngo 1.22\n' >"$1/go.mod"
}

# a. unresolvable ref
d=$tmp/a; newrepo "$d"
if out=$("$script" origin/does-not-exist "$d" "$spec" 2>&1); then bad "a: unresolvable base must fail"
else case $out in *origin/does-not-exist*) ok "a: unresolvable base fails with message";; *) bad "a: message lacks ref: $out";; esac; fi

# b. resolvable base without the spec
d=$tmp/b; newrepo "$d"
sha=$(git -C "$d" rev-parse HEAD)
addspec "$d"
if out=$(CONTRACT_BASE_REQUIRED=0 "$script" HEAD "$d" "$spec" 2>&1); then
	case $out in *"$sha"*"nothing to compare"*) ok "b1: explicit CONTRACT_BASE_REQUIRED=0 skips with SHA";; *) bad "b1: skip message lacks SHA: $out";; esac
else bad "b1: expected explicit skip, got failure: $out"; fi
if out=$(CONTRACT_BASE_REQUIRED=1 "$script" HEAD "$d" "$spec" 2>&1); then bad "b2: absent spec with CONTRACT_BASE_REQUIRED=1 must fail"
else case $out in *"$sha"*"refusing to skip"*) ok "b2: absent spec with requirement on fails";; *) bad "b2: wrong message: $out";; esac; fi
if out=$(env -u CONTRACT_BASE_REQUIRED "$script" HEAD "$d" "$spec" 2>&1); then bad "b3: absent spec must fail by default"
else ok "b3: absent spec fails by default (fail closed)"; fi

# c. base has the spec
d=$tmp/c; newrepo "$d"; addspec "$d"
gitq -C "$d" add -A; gitq -C "$d" commit -q -m spec
if out=$("$script" HEAD "$d" "$spec" 2>&1); then ok "c0: unchanged spec passes against base"
else bad "c0: unchanged spec failed: $out"; fi
# compatible: a new optional operation
python3 -I - "$d/$spec" <<'PY'
import sys
p = sys.argv[1]
s = open(p).read()
marker = "\npaths:\n"
assert marker in s
s = s.replace(marker, marker + "  /probe:\n    get:\n      operationId: probe\n      responses:\n        '204':\n          description: probe\n", 1)
open(p, "w").write(s)
PY
if out=$("$script" HEAD "$d" "$spec" 2>&1); then ok "c1: compatible change (new path) passes"
else bad "c1: compatible change failed: $out"; fi
git -C "$d" checkout -q -- .
# breaking: remove the first path
python3 -I - "$d/$spec" <<'PY'
import re, sys
p = sys.argv[1]
s = open(p).read()
m = re.search(r'^paths:\n(  /[^\n]*:\n(?:    [^\n]*\n|\n)+)', s, re.M)
assert m, "no path found"
open(p, "w").write(s[:m.start(1)] + s[m.end(1):])
PY
if out=$("$script" HEAD "$d" "$spec" 2>&1); then bad "c2: breaking change (path removed) must fail"
else case $out in *"api path removed"*|*"removed"*) ok "c2: breaking change fails in oasdiff";; *) bad "c2: failed for another reason: $out";; esac; fi
git -C "$d" checkout -q -- .
rm -f "$d/$spec"
if out=$("$script" HEAD "$d" "$spec" 2>&1); then bad "c3: deleted spec must fail"
else case $out in *"deleted contract"*) ok "c3: deleted spec fails";; *) bad "c3: wrong message: $out";; esac; fi

# d. git failures that are not "path absent"
d=$tmp/d; newrepo "$d"; addspec "$d"
gitq -C "$d" add -A; gitq -C "$d" commit -q -m spec
blob=$(git -C "$d" rev-parse "HEAD:$spec")
if "$script" "$blob" "$d" "$spec" >/dev/null 2>&1; then bad "d1: non-commit object must fail"; else ok "d1: non-commit object fails"; fi
mkdir -p "$tmp/notgit"
if "$script" HEAD "$tmp/notgit" "$spec" >/dev/null 2>&1; then bad "d2: non-git directory must fail"; else ok "d2: non-git directory fails"; fi
d=$tmp/d3; newrepo "$d"; addspec "$d"
gitq -C "$d" add -A; gitq -C "$d" commit -q -m spec
tree=$(git -C "$d" rev-parse 'HEAD^{tree}')
rm -f "$d/.git/objects/${tree%"${tree#??}"}/${tree#??}"
if git -C "$d" cat-file -e "$tree" 2>/dev/null; then
	echo "SKIP: d3 (tree is packed, cannot corrupt loose object)"
elif "$script" HEAD "$d" "$spec" >/dev/null 2>&1; then bad "d3: missing tree object must fail"; else ok "d3: missing tree object fails (no silent skip)"; fi

if [ "$fails" -ne 0 ]; then echo "$fails test(s) failed"; exit 1; fi
echo "all openapi-breaking tests passed"

#!/bin/sh
# Tests scripts/proto-breaking.sh in throwaway git repos. No network.
# Env: BUF (buf command, default "buf"). Run from the repo root: make proto-breaking-test
set -eu

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/.." && pwd)
script=$here/proto-breaking.sh
BUF=${BUF:-buf}
export BUF
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM
fails=0

gitq() { git -c user.name=t -c user.email=t@example.invalid -c commit.gpgsign=false "$@"; }
ok()   { echo "PASS: $1"; }
bad()  { echo "FAIL: $1"; fails=$((fails + 1)); }

newrepo() { # $1 = dir
	mkdir -p "$1"
	gitq init -q "$1"
	echo x >"$1/README"
	gitq -C "$1" add -A
	gitq -C "$1" commit -q -m base
}
addproto() { # $1 = dir
	mkdir -p "$1/proto"
	cp "$repo/proto/buf.yaml" "$1/proto/"
	cp -R "$repo/proto/sakuraops" "$1/proto/"
	# `go tool -modfile` needs a main module around the working directory.
	[ -f "$1/go.mod" ] || printf 'module probe\n\ngo 1.22\n' >"$1/go.mod"
}
firstmsg() { # file with a message to edit
	find "$1/proto" -name '*.proto' | head -n 1
}

# a. unresolvable ref
d=$tmp/a; newrepo "$d"
if out=$("$script" origin/does-not-exist "$d" 2>&1); then bad "a: unresolvable base must fail"
else case $out in *origin/does-not-exist*) ok "a: unresolvable base fails with message";; *) bad "a: message lacks ref: $out";; esac; fi

# b. resolvable base without proto module
d=$tmp/b; newrepo "$d"
sha=$(git -C "$d" rev-parse HEAD)
addproto "$d"
if out=$(CONTRACT_BASE_REQUIRED=0 "$script" HEAD "$d" 2>&1); then
	case $out in *"$sha"*"nothing to compare"*) ok "b1: explicit CONTRACT_BASE_REQUIRED=0 skips with SHA";; *) bad "b1: skip message lacks SHA: $out";; esac
else bad "b1: expected explicit skip, got failure: $out"; fi
if out=$(CONTRACT_BASE_REQUIRED=1 "$script" HEAD "$d" 2>&1); then bad "b2: absent module with CONTRACT_BASE_REQUIRED=1 must fail"
else case $out in *"$sha"*"refusing to skip"*) ok "b2: absent module with requirement on fails";; *) bad "b2: wrong message: $out";; esac; fi
if out=$(env -u CONTRACT_BASE_REQUIRED "$script" HEAD "$d" 2>&1); then bad "b3: absent module must fail by default"
else ok "b3: absent module fails by default (fail closed)"; fi

# c. post-bootstrap: base has the module
d=$tmp/c; newrepo "$d"; addproto "$d"
gitq -C "$d" add -A; gitq -C "$d" commit -q -m proto
pf=$(firstmsg "$d")
if out=$("$script" HEAD "$d" 2>&1); then ok "c0: unchanged module passes against base"
else bad "c0: unchanged module failed: $out"; fi
printf '\nmessage CompatProbe {\n  string note = 1;\n}\n' >>"$pf"
if out=$("$script" HEAD "$d" 2>&1); then ok "c1: compatible change (new message) passes"
else bad "c1: compatible change failed: $out"; fi
git -C "$d" checkout -q -- .
# breaking: delete the first field of the first message without reserving
python3 -I - "$pf" <<'PY'
import re, sys
p = sys.argv[1]
s = open(p).read()
m = re.search(r'^\s+(?!reserved|option|oneof)[A-Za-z_.<>, ]+? [a-z_0-9]+ = \d+;\n', s, re.M)
assert m, "no field found"
open(p, "w").write(s[:m.start()] + s[m.end():])
PY
if out=$("$script" HEAD "$d" 2>&1); then bad "c2: breaking change (field removed) must fail"
else case $out in *"Previously present field"*) ok "c2: breaking change fails in buf breaking";; *) bad "c2: failed for another reason: $out";; esac; fi

# d. git failures that are not "path absent"
d=$tmp/d; newrepo "$d"; addproto "$d"
gitq -C "$d" add -A; gitq -C "$d" commit -q -m proto
blob=$(git -C "$d" rev-parse HEAD:proto/buf.yaml)
if "$script" "$blob" "$d" >/dev/null 2>&1; then bad "d1: non-commit object must fail"; else ok "d1: non-commit object fails"; fi
mkdir -p "$tmp/notgit"
if "$script" HEAD "$tmp/notgit" >/dev/null 2>&1; then bad "d2: non-git directory must fail"; else ok "d2: non-git directory fails"; fi
d=$tmp/d3; newrepo "$d"; addproto "$d"
gitq -C "$d" add -A; gitq -C "$d" commit -q -m proto
tree=$(git -C "$d" rev-parse 'HEAD^{tree}')
rm -f "$d/.git/objects/${tree%"${tree#??}"}/${tree#??}"
if git -C "$d" cat-file -e "$tree" 2>/dev/null; then
	echo "SKIP: d3 (tree is packed, cannot corrupt loose object)"
elif "$script" HEAD "$d" >/dev/null 2>&1; then bad "d3: missing tree object must fail"; else ok "d3: missing tree object fails (no silent skip)"; fi

if [ "$fails" -ne 0 ]; then echo "$fails test(s) failed"; exit 1; fi
echo "all proto-breaking tests passed"

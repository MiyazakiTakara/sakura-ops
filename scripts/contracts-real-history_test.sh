#!/bin/sh
# Real-history test: proves that proto-breaking and openapi-breaking really execute against the
# integrated base. Clones this repository locally (no network, local objects only) at BASE, then
# checks that the unchanged real contracts pass and deliberately broken copies fail.
# Env: BUF, OASDIFF (commands), PROTO_BASE (default origin/v2/native-agent-blueprint).
# Run from the repo root: make contracts-real-history-test
set -eu

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/.." && pwd)
BUF=${BUF:-buf}
OASDIFF=${OASDIFF:-oasdiff}
export BUF OASDIFF
CONTRACT_BASE_REQUIRED=1
export CONTRACT_BASE_REQUIRED
base=${PROTO_BASE:-origin/v2/native-agent-blueprint}
spec=api/openapi/sakuraops.v1.yaml
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM
fails=0
ok()  { echo "PASS: $1"; }
bad() { echo "FAIL: $1"; fails=$((fails + 1)); }

if ! sha=$(git -C "$repo" rev-parse --verify --quiet "${base}^{commit}"); then
	echo "real-history: ERROR: base '$base' does not resolve (shallow clone?)" >&2
	exit 1
fi
echo "real-history: base $base = $sha"
git clone -q --no-checkout --no-hardlinks "$repo" "$tmp/w"
git -C "$tmp/w" -c advice.detachedHead=false checkout -q "$sha"
# Make the base ref visible inside the clone under the same name (local object copy only).
git -C "$tmp/w" update-ref "refs/remotes/$base" "$sha"
w=$tmp/w
for f in proto/buf.yaml "$spec"; do
	[ -f "$w/$f" ] || { echo "real-history: ERROR: $f missing at base $sha" >&2; exit 1; }
done

run_proto()   { sh "$here/proto-breaking.sh" "$base" "$w" 2>&1; }
run_openapi() { sh "$here/openapi-breaking.sh" "$base" "$w" "$spec" 2>&1; }

# Unchanged tree must pass and must really run (not skip).
if out=$(run_proto); then
	case $out in *"running buf breaking against $base ($sha)"*) ok "proto: unchanged real tree passes and ran buf breaking";; *) bad "proto: no evidence of execution: $out";; esac
else bad "proto: unchanged tree failed: $out"; fi
if out=$(run_openapi); then
	case $out in *"comparing $spec against $base ($sha)"*) ok "openapi: unchanged real tree passes and ran oasdiff";; *) bad "openapi: no evidence of execution: $out";; esac
else bad "openapi: unchanged tree failed: $out"; fi

# Breaking proto change: change the type of the first field in the real file.
pf=$(find "$w/proto" -name '*.proto' | head -n 1)
python3 -I - "$pf" <<'PY'
import re, sys
p = sys.argv[1]
s = open(p).read()
m = re.search(r'^(\s+)(string|bytes|int32|int64|uint32|uint64|bool) ([a-z_0-9]+) = (\d+);', s, re.M)
assert m, "no scalar field found"
new = "bool" if m.group(2) != "bool" else "string"
open(p, "w").write(s[:m.start(2)] + new + s[m.end(2):])
PY
if out=$(run_proto); then bad "proto: breaking change (field type) must fail"
else case $out in *"changed type"*|*"changed cardinality"*|*"changed"*) ok "proto: field type change fails in buf breaking";; *) bad "proto: failed for another reason: $out";; esac; fi
git -C "$w" checkout -q -- .

# Breaking proto change: field number change.
python3 -I - "$pf" <<'PY'
import re, sys
p = sys.argv[1]
s = open(p).read()
m = re.search(r'^(\s+[A-Za-z_.]+ [a-z_0-9]+ = )(\d+)(;)', s, re.M)
assert m, "no field found"
open(p, "w").write(s[:m.start(2)] + str(int(m.group(2)) + 100) + s[m.end(2):])
PY
if out=$(run_proto); then bad "proto: breaking change (field number) must fail"
else case $out in *"Previously present field"*|*"changed"*) ok "proto: field number change fails in buf breaking";; *) bad "proto: failed for another reason: $out";; esac; fi
git -C "$w" checkout -q -- .

# Breaking OpenAPI change: remove the first path.
python3 -I - "$w/$spec" <<'PY'
import re, sys
p = sys.argv[1]
s = open(p).read()
m = re.search(r'^paths:\n(  /[^\n]*:\n(?:    [^\n]*\n|\n)+)', s, re.M)
assert m, "no path found"
open(p, "w").write(s[:m.start(1)] + s[m.end(1):])
PY
if out=$(run_openapi); then bad "openapi: breaking change (path removed) must fail"
else case $out in *removed*) ok "openapi: path removal fails in oasdiff";; *) bad "openapi: failed for another reason: $out";; esac; fi
git -C "$w" checkout -q -- .

if [ "$fails" -ne 0 ]; then echo "$fails test(s) failed"; exit 1; fi
echo "all real-history contract tests passed"

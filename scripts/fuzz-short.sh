#!/bin/sh
# fuzz-short.sh [FUZZTIME]
#
# Discovers every `func FuzzXxx(f *testing.F)` in *_test.go files of the module and runs
# each for FUZZTIME (default 10s), one target at a time (the Go tool fuzzes a single target
# per invocation). Any crasher fails the run; new crashers land in <pkg>/testdata/fuzz/.
# GO is the go command (default "go").
set -eu

fuzztime=${1:-10s}
GO=${GO:-go}
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

targets=$(grep -rE --include='*_test.go' '^func Fuzz[A-Za-z0-9_]*\(' cmd internal 2>/dev/null |
	sed -E 's/^(.*)\/[^\/:]+_test\.go:func (Fuzz[A-Za-z0-9_]*)\(.*/\1 \2/' | sort -u)
if [ -z "$targets" ]; then
	echo "fuzz-short: ERROR: no fuzz targets discovered" >&2
	exit 1
fi

count=0
while read -r dir name; do
	echo "== fuzz $name in ./$dir ($fuzztime)"
	"$GO" test -run '^$' -fuzz "^${name}\$" -fuzztime "$fuzztime" "./$dir"
	count=$((count + 1))
done <<EOT
$targets
EOT
echo "fuzz-short: $count target(s) passed"

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestExitCodes(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	fixable := write(t, dir, "fix.json", `{"source":{},"matches":[{"vulnerability":{"id":"CVE-2099-1","severity":"High","fix":{"versions":["2"],"state":"fixed"}},"artifact":{"name":"p","version":"1"}}]}`)
	unfixed := write(t, dir, "unfixed.json", `{"source":{},"matches":[{"vulnerability":{"id":"CVE-2099-1","severity":"Critical","fix":{"versions":[],"state":"not-fixed"}},"artifact":{"name":"p","version":"1"}}]}`)
	empty := write(t, dir, "empty.json", `{"ignores":[]}`)
	expired := write(t, dir, "expired.json", `{"ignores":[{"id":"CVE-2099-1","image":"hub","package":"p","reason":"r","reviewer":"x","expires":"2026-01-01"}]}`)
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"fixable fails", []string{"-ignore", empty, "hub=" + fixable}, 1},
		{"unfixed warns only", []string{"-ignore", empty, "hub=" + unfixed}, 0},
		{"expired ignore fails", []string{"-ignore", expired, "hub=" + fixable}, 1},
		{"missing report", []string{"-ignore", empty, "hub=" + filepath.Join(dir, "none.json")}, 2},
		{"bad argument", []string{"-ignore", empty, fixable}, 2},
		{"no arguments", nil, 2},
	}
	for _, c := range cases {
		var o, e bytes.Buffer
		if got := run(c.args, &o, &e, now); got != c.want {
			t.Errorf("%s: exit %d, want %d\n%s%s", c.name, got, c.want, o.String(), e.String())
		}
	}
}

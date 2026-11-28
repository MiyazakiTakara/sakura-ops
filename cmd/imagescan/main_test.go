package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

const dg = "sha256:64c8b0d31e4ab6ea665e8426ea406a94bf52de3c05b3a4a8bf713704eefa9328"

func rep(matches string) string {
	return `{"matches":[` + matches + `],"source":{"type":"image","target":{"imageID":"sha256:aaaa","manifestDigest":"","repoDigests":["postgres@` + dg + `"]}},"descriptor":{"name":"grype","version":"0.122.1"}}`
}

func TestExitCodes(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	fixable := write(t, dir, "fix.json", rep(`{"vulnerability":{"id":"CVE-2099-1","severity":"High","fix":{"versions":["2"],"state":"fixed"}},"artifact":{"name":"p","version":"1"}}`))
	unfixed := write(t, dir, "unfixed.json", rep(`{"vulnerability":{"id":"CVE-2099-1","severity":"Critical","fix":{"versions":[],"state":"not-fixed"}},"artifact":{"name":"p","version":"1"}}`))
	clean := write(t, dir, "clean.json", rep(``))
	nomatches := write(t, dir, "nomatches.json", `{"source":{}}`)
	empty := write(t, dir, "empty.json", `{"ignores":[]}`)
	expired := write(t, dir, "expired.json", `{"ignores":[{"id":"CVE-2099-1","image":"hub","digest":"`+dg+`","package":"p","version":"1","reason":"r","reviewer":"x","expires":"2026-01-01"}]}`)
	valid := write(t, dir, "valid.json", `{"ignores":[{"id":"CVE-2099-1","image":"hub","digest":"`+dg+`","package":"p","version":"1","reason":"r","reviewer":"x","expires":"2030-01-01"}]}`)
	wrongVersion := write(t, dir, "wrongver.json", `{"ignores":[{"id":"CVE-2099-1","image":"hub","digest":"`+dg+`","package":"p","version":"2","reason":"r","reviewer":"x","expires":"2030-01-01"}]}`)
	g := []string{"-grype-version", "0.122.1"}
	ext := ",digest=" + dg
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"fixable fails", append(append([]string{"-ignore", empty}, g...), "hub="+fixable+ext), 1},
		{"unfixed warns only", append(append([]string{"-ignore", empty}, g...), "hub="+unfixed+ext), 0},
		{"clean passes", append(append([]string{"-ignore", empty}, g...), "hub="+clean+ext), 0},
		{"expired ignore fails", append(append([]string{"-ignore", expired}, g...), "hub="+fixable+ext), 1},
		{"exact ignore passes", append(append([]string{"-ignore", valid}, g...), "hub="+fixable+ext), 0},
		{"ignore with other version is unused and fails", append(append([]string{"-ignore", wrongVersion}, g...), "hub="+fixable+ext), 1},
		{"wrong digest is an input error", append(append([]string{"-ignore", empty}, g...), "hub="+clean+",digest=sha256:"+strings.Repeat("0", 64)), 2},
		{"report without matches is an input error", append(append([]string{"-ignore", empty}, g...), "hub="+nomatches+ext), 2},
		{"wrong grype version", append([]string{"-ignore", empty, "-grype-version", "9.9.9"}, "hub="+clean+ext), 2},
		{"missing grype version", []string{"-ignore", empty, "hub=" + clean + ext}, 2},
		{"missing report", append(append([]string{"-ignore", empty}, g...), "hub="+filepath.Join(dir, "none.json")+ext), 2},
		{"bad argument", append(append([]string{"-ignore", empty}, g...), fixable), 2},
		{"no identity", append(append([]string{"-ignore", empty}, g...), "hub="+clean), 2},
		{"duplicate label", append(append([]string{"-ignore", empty}, g...), "hub="+clean+ext, "hub="+clean+ext), 2},
		{"no arguments", nil, 2},
	}
	for _, c := range cases {
		var o, e bytes.Buffer
		if got := run(c.args, &o, &e, now); got != c.want {
			t.Errorf("%s: exit %d, want %d\n%s%s", c.name, got, c.want, o.String(), e.String())
		}
	}
}

func TestSummaryShowsCounts(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	unfixed := write(t, dir, "u.json", rep(`{"vulnerability":{"id":"CVE-2099-1","severity":"High","fix":{"versions":[],"state":"not-fixed"}},"artifact":{"name":"p","version":"1"}}`))
	empty := write(t, dir, "e.json", `{"ignores":[]}`)
	var o, e bytes.Buffer
	if got := run([]string{"-ignore", empty, "-grype-version", "0.122.1", "pg=" + unfixed + ",digest=" + dg}, &o, &e, now); got != 0 {
		t.Fatalf("exit %d %s", got, e.String())
	}
	for _, w := range []string{"WARNING (unfixed): pg", "pg: 1 matches, 0 fixable", "1 unfixed HIGH/CRITICAL warnings", "1 distinct HIGH/CRITICAL advisories"} {
		if !strings.Contains(o.String(), w) {
			t.Errorf("output lacks %q:\n%s", w, o.String())
		}
	}
}

func TestPlanMode(t *testing.T) {
	dir := t.TempDir()
	c := write(t, dir, "c.yaml", "services:\n  a:\n    image: postgres@"+dg+"\n  b:\n    image: postgres@"+dg+"\n  hub:\n    build:\n      context: ../..\n      dockerfile: deployment/docker/hub.Dockerfile\n      target: hub\n")
	var o, e bytes.Buffer
	if got := run([]string{"plan", "-compose", c}, &o, &e, time.Now()); got != 0 {
		t.Fatalf("exit %d %s", got, e.String())
	}
	if strings.Count(o.String(), "external\t") != 1 || !strings.Contains(o.String(), "build\thub\thub\thub") {
		t.Errorf("plan: %q", o.String())
	}
	bad := write(t, dir, "bad.yaml", "services:\n  a:\n    image: postgres:18\n")
	o.Reset()
	e.Reset()
	if got := run([]string{"plan", "-compose", bad}, &o, &e, time.Now()); got != 2 || o.Len() != 0 {
		t.Errorf("unpinned: exit %d out %q", got, o.String())
	}
	if got := run([]string{"plan", "-compose", filepath.Join(dir, "none")}, &o, &e, time.Now()); got != 2 {
		t.Errorf("missing file: exit %d", got)
	}
}

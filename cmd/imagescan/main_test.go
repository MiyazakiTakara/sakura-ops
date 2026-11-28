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

const syftOK = `{"artifacts":[{"name":"stdlib","version":"go1.24.6"}],"source":{"type":"image","metadata":{"imageID":"sha256:aaaa","manifestDigest":"","repoDigests":["postgres@` + dg + `"]}},"descriptor":{"name":"syft","version":"1.57.2"}}`

func repWithDigest(digest, matches string) string {
	return `{"matches":[` + matches + `],"source":{"type":"image","target":{"imageID":"sha256:aaaa","manifestDigest":"","repoDigests":["postgres@` + digest + `"]}},"descriptor":{"name":"grype","version":"0.122.1"}}`
}

func match(id, sev, fix, pkg string) string {
	state, versions := "not-fixed", "[]"
	if fix != "" {
		state, versions = "fixed", `["`+fix+`"]`
	}
	return `{"vulnerability":{"id":"` + id + `","severity":"` + sev + `","fix":{"versions":` + versions + `,"state":"` + state + `"}},"artifact":{"name":"` + pkg + `","version":"1"}}`
}

func TestExitCodes(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	fixable := write(t, dir, "fix.json", rep(match("CVE-2099-1", "High", "2", "p")))
	unfixed := write(t, dir, "unfixed.json", rep(match("CVE-2099-1", "Critical", "", "p")))
	clean := write(t, dir, "clean.json", rep(``))
	wrongID := write(t, dir, "wrongid.json", repWithDigest("sha256:"+strings.Repeat("0", 64), ""))
	nomatches := write(t, dir, "nomatches.json", `{"source":{}}`)
	syft := write(t, dir, "syft.json", syftOK)
	syftWrong := write(t, dir, "syftwrong.json", strings.Replace(syftOK, dg, "sha256:"+strings.Repeat("0", 64), 1))
	syftEmpty := write(t, dir, "syftempty.json", strings.Replace(syftOK, `[{"name":"stdlib","version":"go1.24.6"}]`, `[]`, 1))
	empty := write(t, dir, "empty.json", `{"ignores":[]}`)
	expired := write(t, dir, "expired.json", `{"ignores":[{"id":"CVE-2099-1","image":"hub","digest":"`+dg+`","package":"p","version":"1","reason":"r","reviewer":"x","expires":"2026-01-01"}]}`)
	valid := write(t, dir, "valid.json", `{"ignores":[{"id":"CVE-2099-1","image":"hub","digest":"`+dg+`","package":"p","version":"1","reason":"r","reviewer":"x","expires":"2030-01-01"}]}`)
	wrongVersion := write(t, dir, "wrongver.json", `{"ignores":[{"id":"CVE-2099-1","image":"hub","digest":"`+dg+`","package":"p","version":"2","reason":"r","reviewer":"x","expires":"2030-01-01"}]}`)
	g := []string{"-grype-version", "0.122.1", "-syft-version", "1.57.2"}
	ext := ",digest=" + dg
	img := func(direct, sbom, syftf string) string {
		return "hub=" + direct + ",sbom-report=" + sbom + ",syft=" + syftf + ext
	}
	both := func(f string) string { return img(f, f, syft) }
	with := func(ign string, a ...string) []string { return append(append([]string{"-ignore", ign}, g...), a...) }
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"fixable fails", with(empty, both(fixable)), 1},
		{"fixable only in sbom mode fails", with(empty, img(clean, fixable, syft)), 1},
		{"fixable only in direct mode fails", with(empty, img(fixable, clean, syft)), 1},
		{"unfixed warns only", with(empty, both(unfixed)), 0},
		{"clean passes", with(empty, both(clean)), 0},
		{"expired ignore fails", with(expired, both(fixable)), 1},
		{"exact ignore passes", with(valid, both(fixable)), 0},
		{"ignore with other version is unused and fails", with(wrongVersion, both(fixable)), 1},
		{"wrong digest is an input error", with(empty, "hub="+clean+",sbom-report="+clean+",syft="+syft+",digest=sha256:"+strings.Repeat("0", 64)), 2},
		{"sbom-mode report with wrong identity is an input error", with(empty, img(clean, wrongID, syft)), 2},
		{"direct report with wrong identity is an input error", with(empty, img(wrongID, clean, syft)), 2},
		{"syft SBOM with wrong identity is an input error", with(empty, img(clean, clean, syftWrong)), 2},
		{"empty syft SBOM is an input error", with(empty, img(clean, clean, syftEmpty)), 2},
		{"missing sbom report argument is an input error", with(empty, "hub="+clean+",syft="+syft+ext), 2},
		{"missing syft argument is an input error", with(empty, "hub="+clean+",sbom-report="+clean+ext), 2},
		{"missing sbom report file is an input error", with(empty, img(clean, filepath.Join(dir, "none.json"), syft)), 2},
		{"missing syft file is an input error", with(empty, img(clean, clean, filepath.Join(dir, "none.json"))), 2},
		{"missing direct report file is an input error", with(empty, img(filepath.Join(dir, "none.json"), clean, syft)), 2},
		{"sbom report without matches is an input error", with(empty, img(clean, nomatches, syft)), 2},
		{"report without matches is an input error", with(empty, both(nomatches)), 2},
		{"wrong grype version", []string{"-ignore", empty, "-grype-version", "9.9.9", "-syft-version", "1.57.2", both(clean)}, 2},
		{"wrong syft version", []string{"-ignore", empty, "-grype-version", "0.122.1", "-syft-version", "9.9.9", both(clean)}, 2},
		{"missing grype version", []string{"-ignore", empty, "-syft-version", "1.57.2", both(clean)}, 2},
		{"missing syft version", []string{"-ignore", empty, "-grype-version", "0.122.1", both(clean)}, 2},
		{"bad argument", with(empty, fixable), 2},
		{"no identity", with(empty, "hub="+clean+",sbom-report="+clean+",syft="+syft), 2},
		{"duplicate label", with(empty, both(clean), both(clean)), 2},
		{"no arguments", nil, 2},
	}
	for _, c := range cases {
		var o, e bytes.Buffer
		if got := run(c.args, &o, &e, now); got != c.want {
			t.Errorf("%s: exit %d, want %d\n%s%s", c.name, got, c.want, o.String(), e.String())
		}
	}
}

func TestSummaryShowsBothModes(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	direct := write(t, dir, "d.json", rep(match("CVE-2099-1", "High", "", "p")+","+match("CVE-2099-2", "Low", "", "q")))
	sbom := write(t, dir, "s.json", rep(match("CVE-2099-1", "High", "", "p")+","+match("CVE-2099-3", "High", "", "r")))
	syft := write(t, dir, "syft.json", syftOK)
	empty := write(t, dir, "e.json", `{"ignores":[]}`)
	var o, e bytes.Buffer
	args := []string{"-ignore", empty, "-grype-version", "0.122.1", "-syft-version", "1.57.2", "pg=" + direct + ",sbom-report=" + sbom + ",syft=" + syft + ",digest=" + dg}
	if got := run(args, &o, &e, now); got != 0 {
		t.Fatalf("exit %d %s", got, e.String())
	}
	for _, w := range []string{"WARNING (unfixed): pg", "pg: direct 2 matches (1 HIGH/CRITICAL), sbom 2 matches (2 HIGH/CRITICAL), union 2 distinct HIGH/CRITICAL: 0 fixable, 2 unfixed warnings", "2 distinct HIGH/CRITICAL advisories"} {
		if !strings.Contains(o.String(), w) {
			t.Errorf("output lacks %q:\n%s", w, o.String())
		}
	}
	if strings.Count(o.String(), "WARNING (unfixed): pg HIGH CVE-2099-1") != 1 {
		t.Errorf("a finding present in both modes must be reported once:\n%s", o.String())
	}
}

func TestPlanMode(t *testing.T) {
	dir := t.TempDir()
	c := write(t, dir, "c.yaml", "services:\n  a:\n    image: postgres@"+dg+"\n  b:\n    image: postgres@"+dg+"\n  hub:\n    build:\n      context: ../..\n      dockerfile: deployment/docker/hub.Dockerfile\n      target: hub\n")
	var o, e bytes.Buffer
	if got := run([]string{"plan", "-compose", c}, &o, &e, time.Now()); got != 0 {
		t.Fatalf("exit %d %s", got, e.String())
	}
	if strings.Count(o.String(), "external\t") != 1 || !strings.Contains(o.String(), "build\thub\tdeployment/docker/hub.Dockerfile\thub\thub") {
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

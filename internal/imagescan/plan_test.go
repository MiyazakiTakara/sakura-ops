package imagescan

import (
	"bytes"
	"strings"
	"testing"
)

const (
	d1 = "sha256:64c8b0d31e4ab6ea665e8426ea406a94bf52de3c05b3a4a8bf713704eefa9328"
	d2 = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
)

func TestParsePlan(t *testing.T) {
	good := `
services:
  postgres:
    image: postgres@` + d1 + `
  db-bootstrap:
    image: postgres@` + d1 + `
  migrate:
    build:
      context: ../..
      target: migrator
    image: local-migrate:local
  hub:
    build:
      context: ../..
      target: hub
    image: local-hub:local
`
	p, err := ParsePlan([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.External) != 1 || p.External[0].Label != "postgres" || p.External[0].Digest != d1 {
		t.Errorf("duplicate identical reference must give one scan: %+v", p.External)
	}
	if len(p.Builds) != 2 || p.Builds[0].Label != "hub" || p.Builds[1].Label != "migrator" || p.Builds[1].Service != "migrate" {
		t.Errorf("build targets: %+v", p.Builds)
	}
	var out bytes.Buffer
	p.Write(&out)
	want := "external\tpostgres\tpostgres@" + d1 + "\t" + d1 + "\nbuild\thub\thub\thub\nbuild\tmigrator\tmigrator\tmigrate\n"
	if out.String() != want {
		t.Errorf("plan output:\n%q\nwant\n%q", out.String(), want)
	}

	// Base tree shape: only hub, no build target (default/last stage).
	p, err = ParsePlan([]byte("services:\n  postgres:\n    image: postgres@" + d1 + "\n  hub:\n    build:\n      context: ../..\n      dockerfile: x\n"))
	if err != nil || len(p.Builds) != 1 || p.Builds[0].Target != "" || p.Builds[0].Label != "hub" {
		t.Fatalf("%v %+v", err, p)
	}
	var o2 bytes.Buffer
	p.Write(&o2)
	if !strings.Contains(o2.String(), "build\thub\t-\thub\n") {
		t.Errorf("empty target must print as '-': %q", o2.String())
	}
	// Short build form.
	if p, err = ParsePlan([]byte("services:\n  hub:\n    build: .\n")); err != nil || len(p.Builds) != 1 {
		t.Errorf("short build form: %v %+v", err, p)
	}
}

func TestParsePlanRejects(t *testing.T) {
	cases := map[string]string{
		"conflicting digests":     "services:\n  a:\n    image: postgres@" + d1 + "\n  b:\n    image: postgres@" + d2 + "\n",
		"tag only":                "services:\n  a:\n    image: postgres:18\n",
		"latest":                  "services:\n  a:\n    image: postgres\n",
		"tag and digest":          "services:\n  a:\n    image: postgres:18@" + d1 + "\n",
		"short digest":            "services:\n  a:\n    image: postgres@sha256:abc\n",
		"uppercase digest":        "services:\n  a:\n    image: postgres@sha256:" + strings.Repeat("A", 64) + "\n",
		"neither image nor build": "services:\n  a:\n    restart: no\n",
		"no services key":         "name: x\n",
		"empty services":          "services: {}\n",
		"null services":           "services:\n",
		"not yaml":                "services: [\n",
		"empty file":              "",
		"label collision":         "services:\n  a:\n    image: x/postgres@" + d1 + "\n  b:\n    image: y/postgres@" + d2 + "\n",
		"duplicate target":        "services:\n  a:\n    build:\n      target: t\n  b:\n    build:\n      target: t\n",
		"unpinned next to pinned": "services:\n  a:\n    image: postgres@" + d1 + "\n  b:\n    image: redis:7\n",
	}
	for name, in := range cases {
		if _, err := ParsePlan([]byte(in)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

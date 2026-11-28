package imagescan

import (
	"bytes"
	"os"
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
      dockerfile: deployment/docker/hub.Dockerfile
      target: migrator
    image: local-migrate:local
  hub:
    build:
      context: ../..
      dockerfile: deployment/docker/hub.Dockerfile
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
	want := "external\tpostgres\tpostgres@" + d1 + "\t" + d1 + "\nbuild\thub\tdeployment/docker/hub.Dockerfile\thub\thub\nbuild\tmigrator\tdeployment/docker/hub.Dockerfile\tmigrator\tmigrate\n"
	if out.String() != want {
		t.Errorf("plan output:\n%q\nwant\n%q", out.String(), want)
	}

	// Base tree shape: only hub, no build target (default/last stage).
	p, err = ParsePlan([]byte("services:\n  postgres:\n    image: postgres@" + d1 + "\n  hub:\n    build:\n      context: ../../\n      dockerfile: ./deployment/docker/hub.Dockerfile\n"))
	if err != nil || len(p.Builds) != 1 || p.Builds[0].Target != "" || p.Builds[0].Label != "hub" {
		t.Fatalf("%v %+v", err, p)
	}
	var o2 bytes.Buffer
	p.Write(&o2)
	if !strings.Contains(o2.String(), "build\thub\tdeployment/docker/hub.Dockerfile\t-\thub\n") {
		t.Errorf("empty target must print as '-': %q", o2.String())
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
		"short build form":        "services:\n  hub:\n    build: ../..\n",
		"wrong dockerfile":        "services:\n  hub:\n    build:\n      context: ../..\n      dockerfile: other.Dockerfile\n",
		"missing dockerfile":      "services:\n  hub:\n    build:\n      context: ../..\n",
		"wrong context":           "services:\n  hub:\n    build:\n      context: .\n      dockerfile: deployment/docker/hub.Dockerfile\n",
		"missing context":         "services:\n  hub:\n    build:\n      dockerfile: deployment/docker/hub.Dockerfile\n",
		"escaping dockerfile":     "services:\n  hub:\n    build:\n      context: ../..\n      dockerfile: ../x/../deployment/docker/hub.Dockerfile\n",
		"label collision":         "services:\n  a:\n    image: x/postgres@" + d1 + "\n  b:\n    image: y/postgres@" + d2 + "\n",
		"label collision builds":  "services:\n  a:\n    build:\n      context: ../..\n      dockerfile: deployment/docker/hub.Dockerfile\n      target: postgres\n  b:\n    build:\n      context: ../..\n      dockerfile: deployment/docker/postgres.Dockerfile\n",
		"unlisted build arg":      "services:\n  pg:\n    build:\n      context: ../..\n      dockerfile: deployment/docker/postgres.Dockerfile\n      args:\n        BASE: evil\n",
		"traversal to postgres":   "services:\n  pg:\n    build:\n      context: ../..\n      dockerfile: ../../deployment/docker/postgres.Dockerfile\n",
		"other dir same name":     "services:\n  pg:\n    build:\n      context: ../..\n      dockerfile: x/deployment/docker/postgres.Dockerfile\n",
		"unpinned next to pinned": "services:\n  a:\n    image: postgres@" + d1 + "\n  b:\n    image: redis:7\n",
	}
	for name, in := range cases {
		if _, err := ParsePlan([]byte(in)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestParsePlanDerivedPostgres(t *testing.T) {
	in := `
services:
  postgres:
    build:
      context: ../..
      dockerfile: deployment/docker/postgres.Dockerfile
      args:
        POSTGRES_UID: "1000"
        POSTGRES_GID: "1000"
    image: sakuraops-dev-postgres:local
  db-bootstrap:
    build:
      context: ../..
      dockerfile: deployment/docker/postgres.Dockerfile
      args:
        POSTGRES_UID: "1000"
        POSTGRES_GID: "1000"
    image: sakuraops-dev-postgres:local
  hub:
    build:
      context: ../..
      dockerfile: deployment/docker/hub.Dockerfile
      target: hub
`
	p, err := ParsePlan([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.External) != 0 || len(p.Builds) != 2 {
		t.Fatalf("%+v", p)
	}
	// Sorted by service name: db-bootstrap claims the shared build.
	if b := p.Builds[0]; b.Label != "postgres" || b.Dockerfile != "deployment/docker/postgres.Dockerfile" || b.Target != "" || b.Service != "db-bootstrap" {
		t.Errorf("postgres build: %+v", b)
	}
}

// The compose file in the repository must itself be a valid plan with no
// external image left: postgres is the derived, locally built image.
func TestRepositoryComposePlan(t *testing.T) {
	b, err := os.ReadFile("../../deployment/compose/compose.dev.yaml")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParsePlan(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.External) != 0 {
		t.Errorf("external images remain in compose: %+v", p.External)
	}
	got := map[string]string{}
	for _, b := range p.Builds {
		got[b.Label] = b.Dockerfile + "#" + b.Target
	}
	want := map[string]string{
		"postgres": "deployment/docker/postgres.Dockerfile#",
		"hub":      "deployment/docker/hub.Dockerfile#hub",
		"migrator": "deployment/docker/hub.Dockerfile#migrator",
	}
	if len(got) != len(want) {
		t.Fatalf("builds %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("build %s = %q, want %q", k, got[k], v)
		}
	}
}

func TestRepositoryDockerfilesArePinned(t *testing.T) {
	for df := range allowedDockerfiles {
		b, err := os.ReadFile("../../" + df)
		if err != nil {
			t.Fatal(err)
		}
		if err := CheckDockerfilePins(df, b); err != nil {
			t.Errorf("%v", err)
		}
	}
}

func TestPostgresDockerfileRemovesGosu(t *testing.T) {
	b, err := os.ReadFile("../../deployment/docker/postgres.Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, w := range []string{"FROM postgres@" + d1, "rm -f /usr/local/bin/gosu", "\nUSER postgres\n"} {
		if !strings.Contains(s, w) {
			t.Errorf("postgres.Dockerfile lacks %q", w)
		}
	}
}

func TestCheckDockerfilePins(t *testing.T) {
	ok := "# c\nFROM node@" + d1 + " AS web\nFROM golang:1.27@" + d1 + " AS build\nFROM --platform=linux/amd64 x/y@" + d1 + "\nFROM web AS final\n"
	if err := CheckDockerfilePins("ok", []byte(ok)); err != nil {
		t.Errorf("valid file rejected: %v", err)
	}
	bad := map[string]string{
		"tag only":           "FROM postgres:18\n",
		"latest":             "FROM postgres\n",
		"digest not sha256":  "FROM postgres@md5:abc\n",
		"short digest":       "FROM postgres@sha256:abc\n",
		"second FROM tag":    "FROM node@" + d1 + " AS a\nFROM debian:trixie\n",
		"unknown stage":      "FROM node@" + d1 + " AS a\nFROM b\n",
		"variable":           "FROM ${BASE}\n",
		"lowercase from tag": "from postgres:18\n",
		"no FROM":            "RUN true\n",
		"empty":              "",
	}
	for name, in := range bad {
		if err := CheckDockerfilePins(name, []byte(in)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

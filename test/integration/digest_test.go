package integration

import (
	"fmt"
	"os"
	"regexp"
	"testing"
)

// This file has no build tag: it runs in the normal unit test run (make test, CI), so a
// postgres digest bump in only one place is caught without Docker.

var (
	fromImageRe    = regexp.MustCompile(`(?m)^FROM\s+(postgres@sha256:[0-9a-f]{64})\s*$`)
	harnessImageRe = regexp.MustCompile(`(?m)^const pgImage = "(postgres@sha256:[0-9a-f]{64})"\s*$`)
)

// sameImage extracts the pinned postgres base image of the derived Dockerfile
// (deployment/docker/postgres.Dockerfile; Compose builds it, no external postgres image is
// referenced) and compares it with the harness image.
// The error names both values (or what was wrong with finding them).
func sameImage(compose, harness string) error {
	c := fromImageRe.FindAllStringSubmatch(compose, -1)
	h := harnessImageRe.FindAllStringSubmatch(harness, -1)
	if len(c) == 0 {
		return fmt.Errorf("postgres.Dockerfile: want at least one pinned postgres FROM, found none")
	}
	if len(h) != 1 {
		return fmt.Errorf("harness_test.go: want exactly one pgImage const with a pinned digest, found %d", len(h))
	}
	for _, m := range c {
		if m[1] != h[0][1] {
			return fmt.Errorf("postgres digest mismatch: postgres.Dockerfile has %q, harness_test.go pgImage has %q", m[1], h[0][1])
		}
	}
	return nil
}

func TestPostgresDigestMatchesCompose(t *testing.T) {
	compose, err := os.ReadFile("../../deployment/docker/postgres.Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	yml, err := os.ReadFile("../../deployment/compose/compose.dev.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`(?m)^\s*image:\s*postgres[:@]`).Match(yml) {
		t.Fatal("compose.dev.yaml references an external postgres image; it must use the derived image")
	}
	harness, err := os.ReadFile("harness_test.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := sameImage(string(compose), string(harness)); err != nil {
		t.Fatal(err)
	}
}

func TestSameImageDetectsProblems(t *testing.T) {
	a := "postgres@sha256:" + fmt.Sprintf("%064d", 1)
	b := "postgres@sha256:" + fmt.Sprintf("%064d", 2)
	compose := func(img string) string { return "# c\nFROM " + img + "\nUSER postgres\n" }
	harness := func(img string) string { return "package x\n\nconst pgImage = \"" + img + "\"\n" }

	if err := sameImage(compose(a), harness(a)); err != nil {
		t.Fatalf("equal digests rejected: %v", err)
	}
	err := sameImage(compose(a), harness(b))
	if err == nil {
		t.Fatal("differing digests accepted")
	}
	for _, want := range []string{a, b} {
		if !contains(err.Error(), want) {
			t.Errorf("error %q does not name %s", err, want)
		}
	}
	if sameImage("USER postgres\n", harness(a)) == nil {
		t.Error("missing compose image accepted")
	}
	if sameImage(compose(a)+compose(b), harness(a)) == nil {
		t.Error("one of two differing compose images accepted")
	}
	if err := sameImage(compose(a)+compose(a), harness(a)); err != nil {
		t.Errorf("two equal compose images rejected: %v", err)
	}
	if sameImage(compose(a), "package x\n") == nil {
		t.Error("missing pgImage accepted")
	}
	if sameImage(compose("postgres:18-trixie"), harness(a)) == nil {
		t.Error("unpinned tag accepted")
	}
}

func contains(s, sub string) bool { return regexp.MustCompile(regexp.QuoteMeta(sub)).MatchString(s) }

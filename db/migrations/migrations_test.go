package migrations

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
)

func TestEmbeddedMigrations(t *testing.T) {
	t.Parallel()
	name := regexp.MustCompile(`^\d{5}_[a-z0-9_]+\.sql$`)
	entries, err := fs.ReadDir(FS(), ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no embedded migrations")
	}
	for _, e := range entries {
		if !name.MatchString(e.Name()) {
			t.Errorf("migration %q does not match NNNNN_snake_case.sql", e.Name())
		}
		b, err := fs.ReadFile(FS(), e.Name())
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		if !strings.Contains(src, "-- +goose Up") {
			t.Errorf("%s: missing '-- +goose Up'", e.Name())
		}
		// Forward-only policy (docs/dev/database.md): rollback is restore from backup.
		if strings.Contains(src, "-- +goose Down") {
			t.Errorf("%s: Down sections are not allowed", e.Name())
		}
	}
	if got := LatestVersion(); got < 1 {
		t.Fatalf("LatestVersion = %d", got)
	}
}

func TestLatestVersionParsing(t *testing.T) {
	t.Parallel()
	ok := fstest.MapFS{"00001_a.sql": {}, "00007_b.sql": {}, "00003_c.sql": {}, "README.md": {}}
	if v, err := latestVersion(ok); err != nil || v != 7 {
		t.Fatalf("got %d, %v", v, err)
	}
	for name, m := range map[string]fstest.MapFS{
		"empty":     {},
		"no prefix": {"baseline.sql": {}},
		"bad":       {"abc_x.sql": {}},
		"zero":      {"00000_x.sql": {}},
	} {
		if _, err := latestVersion(m); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

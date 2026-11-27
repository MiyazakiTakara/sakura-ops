// Package migrations embeds the forward-only goose migrations. They are applied
// by cmd/sakuraops-migrate and nothing else.
package migrations

import (
	"embed"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
)

//go:embed *.sql
var files embed.FS

// FS returns the embedded migrations (files at the root, NNNNN_name.sql).
func FS() fs.FS { return files }

// LatestVersion is the highest migration version compiled into this binary. The
// hub's readiness check compares the database schema version against it.
func LatestVersion() int64 {
	v, err := latestVersion(files)
	if err != nil {
		// Cannot happen: names are validated by TestEmbeddedMigrations at build time.
		panic("migrations: " + err.Error())
	}
	return v
}

func latestVersion(fsys fs.FS) (int64, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return 0, err
	}
	var latest int64
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		num, _, ok := strings.Cut(name, "_")
		if !ok {
			return 0, fmt.Errorf("migration %q has no NNNNN_ prefix", name)
		}
		v, err := strconv.ParseInt(num, 10, 64)
		if err != nil || v < 1 {
			return 0, fmt.Errorf("migration %q has an invalid version prefix", name)
		}
		latest = max(latest, v)
	}
	if latest == 0 {
		return 0, fmt.Errorf("no migrations found")
	}
	return latest, nil
}

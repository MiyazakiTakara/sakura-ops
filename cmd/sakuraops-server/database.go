package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"

	"github.com/MiyazakiTakara/sakura-ops/db/migrations"
	"github.com/MiyazakiTakara/sakura-ops/internal/httpserver"
	"github.com/MiyazakiTakara/sakura-ops/internal/store"
)

// databaseURLFile is optional: without it the hub runs without a database and
// /readyz answers 503 db_not_configured. The URL itself is never a flag value,
// only the path of a private file that holds it.
var databaseURLFile = flag.String("database-url-file", "",
	"file with the database URL for the DML-only sakuraops_app role (enables /readyz)")

// databaseOptions opens the pool when -database-url-file is set and returns the
// handler options plus a closer. The pool connects lazily, so an unreachable
// database does not stop the hub from starting; /readyz reports it instead.
func databaseOptions(ctx context.Context, path string, log *slog.Logger) ([]httpserver.Option, func(), error) {
	if path == "" {
		return nil, func() {}, nil
	}
	st, warnings, err := store.OpenFromFile(ctx, path, store.Options{ApplicationName: "sakuraops-hub"})
	if err != nil {
		return nil, nil, fmt.Errorf("database: %w", err)
	}
	for _, w := range warnings {
		log.Warn(w)
	}
	required := migrations.LatestVersion()
	log.Info("database configured", "required_schema_version", required)
	ready := httpserver.DBReadiness{Store: st, Required: required, Log: log}
	return []httpserver.Option{httpserver.WithReadiness(ready)}, st.Close, nil
}

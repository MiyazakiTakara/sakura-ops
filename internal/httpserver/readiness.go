package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/MiyazakiTakara/sakura-ops/internal/store"
)

// ErrSchemaOutdated is returned by a Readiness when the database is reachable but
// migrated to an older schema than this binary requires.
var ErrSchemaOutdated = errors.New("database schema is older than this binary requires")

// ErrSchemaAhead is returned when the database was migrated by a newer release
// than this binary: an old binary must not run on a schema it does not know.
var ErrSchemaAhead = errors.New("database schema is newer than this binary supports")

// Readiness reports whether the hub's dependencies can serve traffic. Any error
// other than ErrSchemaOutdated or ErrSchemaAhead means the dependency is unavailable.
type Readiness interface {
	Ready(ctx context.Context) (schemaVersion int64, err error)
}

const readyTimeout = 3 * time.Second

func (s *server) readyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w, "GET, HEAD")
		return
	}
	if s.ready == nil {
		writeError(w, http.StatusServiceUnavailable, "db_not_configured", "Database is not configured.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
	defer cancel()
	v, err := s.ready.Ready(ctx)
	switch {
	case errors.Is(err, ErrSchemaOutdated):
		writeError(w, http.StatusServiceUnavailable, "schema_outdated", "Database schema is not up to date.")
	case errors.Is(err, ErrSchemaAhead):
		writeError(w, http.StatusServiceUnavailable, "schema_ahead", "Database schema is newer than this binary supports.")
	case err != nil:
		writeError(w, http.StatusServiceUnavailable, "db_unavailable", "Database is unavailable.")
	default:
		writeJSON(w, http.StatusOK, struct {
			Status        string `json:"status"`
			SchemaVersion int64  `json:"schema_version"`
		}{"ready", v})
	}
}

// SchemaVersioner is what the database readiness check needs from the store.
type SchemaVersioner interface {
	SchemaVersion(ctx context.Context) (int64, error)
}

// DBReadiness checks that the database answers and that its schema version is at
// exactly Required. Failures are logged as a redacted classification only; the
// HTTP response carries no driver detail either.
type DBReadiness struct {
	Store    SchemaVersioner
	Required int64
	Log      *slog.Logger
}

// Ready implements Readiness.
func (d DBReadiness) Ready(ctx context.Context) (int64, error) {
	v, err := d.Store.SchemaVersion(ctx)
	if err != nil {
		if d.Log != nil {
			d.Log.Error("readiness: database check failed", "cause", store.Describe(err))
		}
		return 0, err
	}
	if v < d.Required {
		if d.Log != nil {
			d.Log.Error("readiness: schema outdated", "have", v, "need", d.Required)
		}
		return v, ErrSchemaOutdated
	}
	if v > d.Required {
		if d.Log != nil {
			d.Log.Error("readiness: schema ahead of binary", "have", v, "need", d.Required)
		}
		return v, ErrSchemaAhead
	}
	return v, nil
}

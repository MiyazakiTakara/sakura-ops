// Package migrate runs the embedded goose migrations with a PostgreSQL session
// advisory lock. Forward-only: there is no Down operation here by design.
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/MiyazakiTakara/sakura-ops/internal/store"
)

// VersionTable is the goose bookkeeping table. It lives in the sakuraops schema,
// is owned by the migrator role and is not accessible to the application role.
const VersionTable = store.Schema + ".goose_db_version"

// lockID is the advisory lock key shared by every SakuraOps migrator. Arbitrary
// but fixed ("sakuraops" in ASCII-ish digits); do not reuse it elsewhere.
const lockID int64 = 0x5341_4b55_5241

// Runner applies and reports migrations on one database.
type Runner struct {
	db       *sql.DB
	provider *goose.Provider
}

// OpenDB opens a database/sql handle for the migrator from a database URL. The
// search_path is pinned to the sakuraops schema.
func OpenDB(dsn string, applicationName string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, store.ErrInvalidDSN
	}
	cfg.RuntimeParams["application_name"] = applicationName
	cfg.RuntimeParams["search_path"] = store.Schema
	return stdlib.OpenDB(*cfg), nil
}

// New builds a Runner over db and the given migration files (use
// migrations.FS() in production; tests pass fixture sets).
func New(db *sql.DB, fsys fs.FS) (*Runner, error) {
	locker, err := lock.NewPostgresSessionLocker(
		lock.WithLockID(lockID),
		// Poll every second for up to 2 minutes while a concurrent migrator runs.
		lock.WithLockTimeout(1, 120),
	)
	if err != nil {
		return nil, fmt.Errorf("create session locker: %w", err)
	}
	p, err := goose.NewProvider(goose.DialectPostgres, db, fsys,
		goose.WithTableName(VersionTable),
		goose.WithSessionLocker(locker),
	)
	if err != nil {
		return nil, fmt.Errorf("create goose provider: %w", err)
	}
	return &Runner{db: db, provider: p}, nil
}

// Close closes the provider and the underlying handle.
func (r *Runner) Close() error {
	return errors.Join(r.provider.Close(), r.db.Close())
}

// Applied describes one migration applied by Up.
type Applied struct {
	Version int64
	Source  string
}

// Up applies all pending migrations under the advisory lock. An empty result
// means the database was already current.
func (r *Runner) Up(ctx context.Context) ([]Applied, error) {
	res, err := r.provider.Up(ctx)
	out := make([]Applied, 0, len(res))
	for _, m := range res {
		out = append(out, Applied{Version: m.Source.Version, Source: m.Source.Path})
	}
	if err != nil {
		return out, fmt.Errorf("apply migrations: %w", err)
	}
	return out, nil
}

// Item is one known migration and whether the database has it.
type Item struct {
	Version int64
	Source  string
	Applied bool
}

// Status lists every embedded migration with its applied state. It creates the
// goose table when missing, which requires the migrator role.
func (r *Runner) Status(ctx context.Context) ([]Item, error) {
	st, err := r.provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("read migration status: %w", err)
	}
	items := make([]Item, 0, len(st))
	for _, s := range st {
		items = append(items, Item{
			Version: s.Source.Version,
			Source:  s.Source.Path,
			Applied: s.State == goose.StateApplied,
		})
	}
	return items, nil
}

// Version returns the database's current version and the latest known version.
func (r *Runner) Version(ctx context.Context) (current, latest int64, err error) {
	current, latest, err = r.provider.GetVersions(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("read versions: %w", err)
	}
	return current, latest, nil
}

// Command sakuraops-migrate applies the embedded, forward-only database
// migrations. Run it as a one-shot job with the sakuraops_migrator role.
//
//	sakuraops-migrate [-database-url-file PATH] [-timeout 2m] up|status|version
//
// The database URL is read only from a file (flag or SAKURAOPS_DATABASE_URL_FILE),
// never from the command line, and is never printed. There is no "down":
// rollback is restore from backup (docs/dev/database.md).
//
// Exit codes: 0 success; 1 database or migration failure (including timeout);
// 2 usage error; 3 configuration error (URL file missing, unsafe or invalid).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MiyazakiTakara/sakura-ops/db/migrations"
	"github.com/MiyazakiTakara/sakura-ops/internal/dsnfile"
	"github.com/MiyazakiTakara/sakura-ops/internal/migrate"
	"github.com/MiyazakiTakara/sakura-ops/internal/store"
)

const (
	exitOK     = 0
	exitFail   = 1
	exitUsage  = 2
	exitConfig = 3

	envURLFile = "SAKURAOPS_DATABASE_URL_FILE"
)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sakuraops-migrate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	urlFile := fs.String("database-url-file", "", "file containing the database URL (default: $"+envURLFile+")")
	timeout := fs.Duration("timeout", 2*time.Minute, "overall deadline for the command")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: sakuraops-migrate [-database-url-file PATH] [-timeout D] up|status|version\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return exitUsage
	}
	cmd := fs.Arg(0)
	switch cmd {
	case "up", "status", "version":
	default:
		fmt.Fprintf(stderr, "unknown command %q (down is intentionally not supported)\n", cmd)
		return exitUsage
	}
	if *timeout <= 0 {
		fmt.Fprintln(stderr, "-timeout must be positive")
		return exitUsage
	}

	path := *urlFile
	if path == "" {
		path = getenv(envURLFile)
	}
	log := slog.New(slog.NewTextHandler(stderr, nil))
	if path == "" {
		fmt.Fprintf(stderr, "no database URL file: use -database-url-file or %s\n", envURLFile)
		return exitConfig
	}
	res, err := dsnfile.Read(path)
	if err != nil {
		log.Error("database URL file rejected", "err", err)
		return exitConfig
	}
	for _, w := range res.Warnings {
		log.Warn(w)
	}

	db, err := migrate.OpenDB(res.URL, "sakuraops-migrate")
	if err != nil {
		log.Error("database URL invalid")
		return exitConfig
	}
	r, err := migrate.New(db, migrations.FS())
	if err != nil {
		log.Error("migrator setup failed", "err", err)
		_ = db.Close()
		return exitFail
	}
	defer r.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	if err := execute(ctx, r, cmd, stdout); err != nil {
		// Never print the raw error: driver messages can name hosts and users.
		attrs := []any{"command", cmd, "cause", store.Describe(err)}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && !connectionClass(pgErr.Code) {
			// Server-side message only (no detail, no row data, no connection info).
			attrs = append(attrs, "server_message", pgErr.Message)
		}
		log.Error("command failed", attrs...)
		return exitFail
	}
	return exitOK
}

// connectionClass reports SQLSTATE classes whose server message echoes connection
// details (28 invalid authorization: "password authentication failed for user ...",
// 3D invalid catalog: database name, 08 connection exception, 57 operator intervention).
func connectionClass(code string) bool {
	if len(code) < 2 {
		return true
	}
	switch code[:2] {
	case "08", "28", "3D", "57":
		return true
	}
	return false
}

func execute(ctx context.Context, r *migrate.Runner, cmd string, out io.Writer) error {
	switch cmd {
	case "up":
		applied, err := r.Up(ctx)
		for _, a := range applied {
			fmt.Fprintf(out, "applied %05d %s\n", a.Version, a.Source)
		}
		if err != nil {
			return err
		}
		if len(applied) == 0 {
			fmt.Fprintln(out, "no migrations to apply")
		}
	case "status":
		items, err := r.Status(ctx)
		if err != nil {
			return err
		}
		for _, it := range items {
			state := "pending"
			if it.Applied {
				state = "applied"
			}
			fmt.Fprintf(out, "%05d %-8s %s\n", it.Version, state, it.Source)
		}
	case "version":
		cur, latest, err := r.Version(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "current %d latest %d\n", cur, latest)
	}
	return nil
}

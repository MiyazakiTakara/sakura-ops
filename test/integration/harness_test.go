//go:build integration

// Package integration runs the migration and readiness tests against a throwaway
// PostgreSQL container (docker): random 127.0.0.1 port, random passwords, data on
// tmpfs, removed afterwards. Run with `make test-integration`.
package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// pgImage must stay identical to the postgres image pinned in
// deployment/compose/compose.dev.yaml (postgres:18-trixie, PostgreSQL 18).
const pgImage = "postgres@sha256:64c8b0d31e4ab6ea665e8426ea406a94bf52de3c05b3a4a8bf713704eefa9328"

const (
	roleMigrator = "sakuraops_migrator"
	roleApp      = "sakuraops_app"
)

var (
	repoRoot     string // module root
	migrateBin   string // built cmd/sakuraops-migrate
	serverBin    string // built cmd/sakuraops-server
	sharedPG     *cluster
	bootstrapSQL []byte
)

type cluster struct {
	name    string
	port    string
	superPW string
	migPW   string
	appPW   string
	stopped bool
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func docker(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return out.Bytes(), fmt.Errorf("docker %s: %w: %s", args[0], err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

// startCluster launches a throwaway PostgreSQL and waits until it accepts logins.
func startCluster() (*cluster, error) {
	c := &cluster{name: "sakuraops-it-" + randHex(4), superPW: randHex(16), migPW: randHex(16), appPW: randHex(16)}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// The superuser password goes through a 0600 env file, not argv.
	envFile := filepath.Join(os.TempDir(), c.name+".env")
	if err := os.WriteFile(envFile, []byte("POSTGRES_PASSWORD="+c.superPW+"\n"), 0o600); err != nil {
		return nil, err
	}
	defer os.Remove(envFile)

	if _, err := docker(ctx, nil, "run", "-d", "--rm", "--name", c.name,
		"--env-file", envFile,
		"-p", "127.0.0.1::5432",
		"--tmpfs", "/var/lib/postgresql:rw,size=512m",
		"--cap-drop", "ALL", "--cap-add", "CHOWN", "--cap-add", "DAC_OVERRIDE", "--cap-add", "FOWNER",
		"--cap-add", "SETGID", "--cap-add", "SETUID",
		"--security-opt", "no-new-privileges:true",
		pgImage, "-c", "fsync=off", "-c", "full_page_writes=off"); err != nil {
		return nil, err
	}
	out, err := docker(ctx, nil, "port", c.name, "5432/tcp")
	if err != nil {
		c.remove()
		return nil, err
	}
	first, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	_, c.port, err = net.SplitHostPort(first)
	if err != nil {
		c.remove()
		return nil, fmt.Errorf("parse docker port output: %w", err)
	}
	for {
		conn, err := pgx.Connect(ctx, c.dsn("postgres", "postgres", c.superPW))
		if err == nil {
			_ = conn.Close(ctx)
			return c, nil
		}
		select {
		case <-ctx.Done():
			c.remove()
			return nil, errors.New("postgres container did not become ready")
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (c *cluster) remove() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, _ = docker(ctx, nil, "rm", "-f", c.name)
}

func (c *cluster) stop(t testing.TB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if _, err := docker(ctx, nil, "stop", "-t", "1", c.name); err != nil {
		t.Fatal(err)
	}
	c.stopped = true
}

func (c *cluster) dsn(user, db, password string) string {
	return fmt.Sprintf("postgres://%s:%s@127.0.0.1:%s/%s?sslmode=disable", user, password, c.port, db)
}

func (c *cluster) migratorDSN(db string) string { return c.dsn(roleMigrator, db, c.migPW) }
func (c *cluster) appDSN(db string) string      { return c.dsn(roleApp, db, c.appPW) }

// superConn connects as the container superuser.
func (c *cluster) superConn(t testing.TB, db string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), c.dsn("postgres", db, c.superPW))
	if err != nil {
		t.Fatalf("superuser connect: %v", describe(err))
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

// mustConnect opens a single connection from a URL and closes it at test end.
func mustConnect(t testing.TB, dsn string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", describe(err))
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

func pgxConnect(dsn string) (*pgx.Conn, error) { return pgx.Connect(context.Background(), dsn) }

// newDB creates an empty database and runs db/bootstrap/roles.sql in it.
func (c *cluster) newDB(t testing.TB) string {
	t.Helper()
	db := "it_" + randHex(6)
	admin := c.superConn(t, "postgres")
	if _, err := admin.Exec(context.Background(), "CREATE DATABASE "+db); err != nil {
		t.Fatalf("create database: %v", describe(err))
	}
	c.bootstrap(t, db)
	return db
}

// bootstrap pipes the real roles.sql to psql inside the container, with the
// passwords as psql variables on stdin (never argv).
func (c *cluster) bootstrap(t testing.TB, db string) {
	t.Helper()
	prelude := fmt.Sprintf("\\set migrator_password '%s'\n\\set app_password '%s'\n", c.migPW, c.appPW)
	if out, err := c.bootstrapRaw(db, prelude, nil); err != nil {
		t.Fatalf("bootstrap: %v\n%s", err, out)
	}
}

// bootstrapRaw runs roles.sql with an optional stdin prelude and extra
// `docker exec` environment (KEY=VALUE).
func (c *cluster) bootstrapRaw(db, prelude string, env []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	args := []string{"exec", "-i"}
	for _, e := range env {
		args = append(args, "-e", e)
	}
	args = append(args, c.name, "psql", "-X", "-q", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", db, "-f", "-")
	return docker(ctx, append([]byte(prelude), bootstrapSQL...), args...)
}

// copyFile places a host file into the container's /tmp (root-owned, 0600 from the source).
func (c *cluster) copyFile(t testing.TB, hostPath, name string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dst := "/tmp/" + name
	if _, err := docker(ctx, nil, "cp", hostPath, c.name+":"+dst); err != nil {
		t.Fatal(err)
	}
	return dst
}

// writeDSNFile stores a URL in a 0600 file (the only way the tools accept it).
func writeDSNFile(t testing.TB, dsn string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "database_url")
	if err := os.WriteFile(p, []byte(dsn+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// describe strips connection details from driver errors before they reach test output.
func describe(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return "sqlstate " + pgErr.Code + ": " + pgErr.Message
	}
	return "connection error"
}

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func buildBinary(dir, name, pkg string) (string, error) {
	out := filepath.Join(dir, name)
	cmd := exec.Command("go", "build", "-trimpath", "-o", out, pkg)
	cmd.Dir = repoRoot
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build %s: %w\n%s", pkg, err, b)
	}
	return out, nil
}

func TestMain(m *testing.M) { os.Exit(testMain(m)) }

func testMain(m *testing.M) int {
	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "go env GOMOD:", err)
		return 1
	}
	repoRoot = filepath.Dir(strings.TrimSpace(string(out)))
	if bootstrapSQL, err = os.ReadFile(filepath.Join(repoRoot, "db", "bootstrap", "roles.sql")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir, err := os.MkdirTemp("", "sakuraops-it-bin-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)
	if migrateBin, err = buildBinary(dir, "sakuraops-migrate", "./cmd/sakuraops-migrate"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if serverBin, err = buildBinary(dir, "sakuraops-server", "./cmd/sakuraops-server"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if sharedPG, err = startCluster(); err != nil {
		fmt.Fprintln(os.Stderr, "start postgres:", err)
		return 1
	}
	defer sharedPG.remove()
	return m.Run()
}

// runCmd runs a binary without a shell and with an empty environment.
func runCmd(t testing.TB, timeout time.Duration, bin string, args ...string) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = []string{}
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	var ee *exec.ExitError
	switch {
	case err == nil:
		return 0, buf.String()
	case errors.As(err, &ee):
		return ee.ExitCode(), buf.String()
	default:
		t.Fatalf("run %s: %v", filepath.Base(bin), err)
		return -1, ""
	}
}

func runMigrate(t testing.TB, urlFile string, verb string) (int, string) {
	t.Helper()
	return runCmd(t, 2*time.Minute, migrateBin, "-database-url-file", urlFile, verb)
}

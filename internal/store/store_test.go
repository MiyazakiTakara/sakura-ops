package store

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestConfigFromDSNDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := ConfigFromDSN("postgres://u:pw@127.0.0.1:5432/db?sslmode=disable", Options{})
	if err != nil {
		t.Fatal(err)
	}
	rp := cfg.ConnConfig.RuntimeParams
	if rp["application_name"] != "sakuraops-hub" || rp["search_path"] != Schema {
		t.Fatalf("runtime params: %v", rp)
	}
	if rp["statement_timeout"] != "15000" {
		t.Fatalf("statement_timeout = %q", rp["statement_timeout"])
	}
	if cfg.ConnConfig.ConnectTimeout != 5*time.Second || cfg.MaxConns != 10 || cfg.MinConns != 0 {
		t.Fatalf("pool config: connect=%v max=%d min=%d", cfg.ConnConfig.ConnectTimeout, cfg.MaxConns, cfg.MinConns)
	}
}

func TestConfigFromDSNOptionsAndOverride(t *testing.T) {
	t.Parallel()
	// A search_path or application_name in the URL must not override ours.
	cfg, err := ConfigFromDSN("postgres://u:pw@127.0.0.1/db?search_path=public&application_name=x", Options{
		ApplicationName: "sakuraops-test", StatementTimeout: 2 * time.Second, MaxConns: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	rp := cfg.ConnConfig.RuntimeParams
	if rp["application_name"] != "sakuraops-test" || rp["search_path"] != Schema || rp["statement_timeout"] != "2000" || cfg.MaxConns != 3 {
		t.Fatalf("got %v max=%d", rp, cfg.MaxConns)
	}
}

func TestConfigFromDSNInvalidHidesInput(t *testing.T) {
	t.Parallel()
	_, err := ConfigFromDSN("postgres://u:topsecretpw@host:notaport/db", Options{})
	if !errors.Is(err, ErrInvalidDSN) {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(err.Error(), "topsecretpw") {
		t.Fatal("error leaks the password")
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout to 10.0.0.1:5432" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return false }

func TestDescribeIsSafe(t *testing.T) {
	t.Parallel()
	cases := map[string]error{
		"sqlstate 28P01":     &pgconn.PgError{Code: "28P01", Message: "password authentication failed for user \"u\""},
		"timeout":            fmt.Errorf("x: %w", context.DeadlineExceeded),
		"canceled":           context.Canceled,
		"network timeout":    net.Error(timeoutErr{}),
		"network error":      &net.OpError{Op: "dial", Err: errors.New("connection refused 10.0.0.1")},
		"unclassified error": errors.New("host=10.0.0.1 password=hunter2"),
	}
	for want, err := range cases {
		got := Describe(err)
		if got != want {
			t.Errorf("Describe(%v) = %q, want %q", err, got, want)
		}
		if strings.ContainsAny(got, "0123456789.") && !strings.HasPrefix(got, "sqlstate") {
			t.Errorf("description %q contains address-like text", got)
		}
	}
}

//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"testing/fstest"
	"time"

	"github.com/MiyazakiTakara/sakura-ops/db/migrations"
	"github.com/MiyazakiTakara/sakura-ops/internal/httpserver"
	"github.com/MiyazakiTakara/sakura-ops/internal/store"
)

type readyBody struct {
	Status        string `json:"status"`
	SchemaVersion int64  `json:"schema_version"`
	Code          string `json:"code"`
	Message       string `json:"message"`
}

func getReady(t testing.TB, base string) (int, readyBody, string) {
	t.Helper()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(base + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var b readyBody
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("readyz body %q: %v", raw, err)
	}
	return resp.StatusCode, b, string(raw)
}

func hubHandler(t testing.TB, st *store.Store, required int64, log *slog.Logger) http.Handler {
	t.Helper()
	web := fstest.MapFS{"index.html": {Data: []byte("<html></html>")}}
	return httpserver.New(web, httpserver.WithReadiness(httpserver.DBReadiness{Store: st, Required: required, Log: log}))
}

func openApp(t testing.TB, c *cluster, db string) *store.Store {
	t.Helper()
	st, _, err := store.OpenFromFile(context.Background(), writeDSNFile(t, c.appDSN(db)), store.Options{ApplicationName: "sakuraops-it"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	return st
}

// TestReadyzSchemaStates: unmigrated and too-old schemas are reported as
// schema_outdated, a current one as ready.
func TestReadyzSchemaStates(t *testing.T) {
	c := sharedPG
	db := c.newDB(t) // bootstrapped, not migrated
	st := openApp(t, c, db)
	latest := migrations.LatestVersion()
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))

	srv := httptest.NewServer(hubHandler(t, st, latest, log))
	defer srv.Close()

	code, body, raw := getReady(t, srv.URL)
	if code != 503 || body.Code != "schema_outdated" {
		t.Fatalf("before migration: %d %s", code, raw)
	}

	if rc, out := runMigrate(t, writeDSNFile(t, c.migratorDSN(db)), "up"); rc != 0 {
		t.Fatalf("migrate: %d %s", rc, out)
	}
	code, body, raw = getReady(t, srv.URL)
	if code != 200 || body.Status != "ready" || body.SchemaVersion != latest {
		t.Fatalf("after migration: %d %s", code, raw)
	}

	// A binary that needs a newer schema than the database has.
	newer := httptest.NewServer(hubHandler(t, st, latest+1, log))
	defer newer.Close()
	code, body, raw = getReady(t, newer.URL)
	if code != 503 || body.Code != "schema_outdated" {
		t.Fatalf("db older than binary: %d %s", code, raw)
	}
	for _, leak := range []string{c.port, c.appPW, "127.0.0.1", roleApp} {
		if strings.Contains(raw, leak) || strings.Contains(logs.String(), c.appPW) {
			t.Fatalf("leak of %q in response or log", leak)
		}
	}
}

// TestReadyzDatabaseStopped uses its own container so stopping it cannot disturb
// the shared one.
func TestReadyzDatabaseStopped(t *testing.T) {
	c, err := startCluster()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.remove)
	db := migratedDB(t, c)
	st := openApp(t, c, db)
	var logs bytes.Buffer
	srv := httptest.NewServer(hubHandler(t, st, migrations.LatestVersion(), slog.New(slog.NewTextHandler(&logs, nil))))
	defer srv.Close()

	if code, _, raw := getReady(t, srv.URL); code != 200 {
		t.Fatalf("while running: %d %s", code, raw)
	}
	c.stop(t)
	code, body, raw := getReady(t, srv.URL)
	if code != 503 || body.Code != "db_unavailable" {
		t.Fatalf("with database stopped: %d %s", code, raw)
	}
	for _, leak := range []string{c.port, c.appPW, "127.0.0.1", "refused"} {
		if strings.Contains(raw, leak) || strings.Contains(logs.String(), leak) {
			t.Fatalf("driver detail %q leaked into response or log: %s", leak, logs.String())
		}
	}
	// Liveness is independent of the database.
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("healthz with database down: %v", err)
	}
	resp.Body.Close()
}

// freePort returns a currently free loopback port.
func freePort(t testing.TB) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	_, p, _ := net.SplitHostPort(l.Addr().String())
	return p
}

// syncBuffer is a bytes.Buffer safe for a child process writing while the test reads.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

type hubProc struct {
	base string
	cmd  *exec.Cmd
	out  *syncBuffer
}

func startHub(t testing.TB, args ...string) *hubProc {
	t.Helper()
	port := freePort(t)
	h := &hubProc{base: "http://127.0.0.1:" + port, out: &syncBuffer{}}
	h.cmd = exec.Command(serverBin, append([]string{"-listen", "127.0.0.1:" + port}, args...)...)
	h.cmd.Env = []string{}
	h.cmd.Stdout, h.cmd.Stderr = h.out, h.out
	if err := h.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.stop() })
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := http.Get(h.base + "/healthz"); err == nil {
			resp.Body.Close()
			return h
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("hub did not start: %s", h.out)
	return nil
}

func (h *hubProc) stop() {
	if h.cmd.ProcessState == nil {
		_ = h.cmd.Process.Signal(syscall.SIGTERM)
		_ = h.cmd.Wait()
	}
}

// TestHubBinaryReadyz drives the real hub binary through its flag.
func TestHubBinaryReadyz(t *testing.T) {
	c := sharedPG

	t.Run("without database flag", func(t *testing.T) {
		h := startHub(t)
		code, body, raw := getReady(t, h.base)
		if code != 503 || body.Code != "db_not_configured" {
			t.Fatalf("%d %s", code, raw)
		}
	})

	t.Run("with database flag", func(t *testing.T) {
		db := c.newDB(t)
		appFile := writeDSNFile(t, c.appDSN(db))
		h := startHub(t, "-database-url-file", appFile)
		if code, body, raw := getReady(t, h.base); code != 503 || body.Code != "schema_outdated" {
			t.Fatalf("before migration: %d %s", code, raw)
		}
		if rc, out := runMigrate(t, writeDSNFile(t, c.migratorDSN(db)), "up"); rc != 0 {
			t.Fatalf("migrate: %d %s", rc, out)
		}
		code, body, raw := getReady(t, h.base)
		if code != 200 || body.Status != "ready" || body.SchemaVersion != migrations.LatestVersion() {
			t.Fatalf("after migration: %d %s", code, raw)
		}
		if strings.Contains(h.out.String(), c.appPW) {
			t.Fatal("hub log contains the database password")
		}
	})

	t.Run("unsafe URL file permissions stop the hub", func(t *testing.T) {
		db := c.newDB(t)
		f := writeDSNFile(t, c.appDSN(db))
		if err := os.Chmod(f, 0o644); err != nil {
			t.Fatal(err)
		}
		code, out := runCmd(t, 20*time.Second, serverBin, "-listen", "127.0.0.1:"+freePort(t), "-database-url-file", f)
		if code != 1 || strings.Contains(out, c.appPW) {
			t.Fatalf("exit %d, output %q", code, out)
		}
	})
}

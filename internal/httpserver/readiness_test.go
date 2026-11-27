package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

type fakeVersioner struct {
	v   int64
	err error
}

func (f fakeVersioner) SchemaVersion(context.Context) (int64, error) { return f.v, f.err }

func readyHandler(t *testing.T, r Readiness) http.Handler {
	t.Helper()
	return New(fstest.MapFS{"index.html": {Data: []byte(indexHTML)}}, WithReadiness(r))
}

func TestReadyzNotConfigured(t *testing.T) {
	h := testHandler(t)
	rec := do(h, "GET", "/readyz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
	if e := decodeErr(t, rec); e.Code != "db_not_configured" {
		t.Fatalf("code = %q", e.Code)
	}
}

func TestReadyzStates(t *testing.T) {
	const secretish = "host=10.9.8.7 password=hunter2"
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))
	cases := []struct {
		name     string
		store    fakeVersioner
		wantCode int
		wantErr  string
	}{
		{"ready", fakeVersioner{v: 2}, 200, ""},
		{"equal to required", fakeVersioner{v: 2}, 200, ""},
		{"ahead of binary", fakeVersioner{v: 3}, 503, "schema_ahead"},
		{"outdated", fakeVersioner{v: 1}, 503, "schema_outdated"},
		{"not migrated", fakeVersioner{v: 0}, 503, "schema_outdated"},
		{"unavailable", fakeVersioner{err: errors.New(secretish)}, 503, "db_unavailable"},
		{"deadline", fakeVersioner{err: context.DeadlineExceeded}, 503, "db_unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := readyHandler(t, DBReadiness{Store: tc.store, Required: 2, Log: log})
			rec := do(h, "GET", "/readyz")
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Error("readyz must not be cached")
			}
			if tc.wantErr == "" {
				var ok struct {
					Status        string `json:"status"`
					SchemaVersion int64  `json:"schema_version"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &ok); err != nil {
					t.Fatal(err)
				}
				if ok.Status != "ready" || ok.SchemaVersion != tc.store.v {
					t.Fatalf("body %s", rec.Body)
				}
				return
			}
			if e := decodeErr(t, rec); e.Code != tc.wantErr {
				t.Fatalf("code = %q, want %q", e.Code, tc.wantErr)
			}
			if strings.Contains(rec.Body.String(), "10.9.8.7") || strings.Contains(rec.Body.String(), "hunter2") {
				t.Fatal("response leaks driver detail")
			}
		})
	}
	if strings.Contains(logs.String(), "hunter2") || strings.Contains(logs.String(), "10.9.8.7") {
		t.Fatalf("log leaks driver detail: %s", logs.String())
	}
}

func TestReadyzMethods(t *testing.T) {
	h := readyHandler(t, DBReadiness{Store: fakeVersioner{v: 1}, Required: 1})
	if rec := do(h, "HEAD", "/readyz"); rec.Code != 200 {
		t.Fatalf("HEAD = %d", rec.Code)
	}
	for _, m := range []string{"POST", "PUT", "DELETE"} {
		rec := do(h, m, "/readyz")
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") == "" {
			t.Errorf("%s = %d", m, rec.Code)
		}
	}
}

func TestIsReservedReadyz(t *testing.T) {
	for _, p := range []string{"/readyz", "/readyz/", "/readyz/x", "/READYZ", "/ReadyZ/a/b"} {
		if !isReserved(p) {
			t.Errorf("%q must be reserved", p)
		}
	}
	for _, p := range []string{"/ready", "/readyzz", "/hosts/readyz"} {
		if isReserved(p) {
			t.Errorf("%q must not be reserved", p)
		}
	}
}

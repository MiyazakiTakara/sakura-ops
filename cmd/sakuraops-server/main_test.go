package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeHealth(t *testing.T) {
	t.Parallel()
	status := http.StatusOK
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(status)
	}))
	defer ts.Close()
	addr := ts.Listener.Addr().String()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := probeHealth(ctx, addr); err != nil {
		t.Fatalf("healthy server: %v", err)
	}

	// Wildcard listen address is probed through loopback.
	_, port, _ := net.SplitHostPort(addr)
	if err := probeHealth(ctx, "0.0.0.0:"+port); err != nil {
		t.Fatalf("wildcard host: %v", err)
	}
	if err := probeHealth(ctx, ":"+port); err != nil {
		t.Fatalf("empty host: %v", err)
	}

	status = http.StatusServiceUnavailable
	if err := probeHealth(ctx, addr); err == nil {
		t.Fatal("503 must be unhealthy")
	}
	status = http.StatusFound
	if err := probeHealth(ctx, addr); err == nil {
		t.Fatal("redirect must be unhealthy")
	}
}

func TestProbeHealthFailures(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := probeHealth(ctx, "not-an-address"); err == nil {
		t.Fatal("malformed address must fail")
	}
	// Closed port: listen, take the address, close.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	if err := probeHealth(ctx, addr); err == nil {
		t.Fatal("closed port must fail")
	}
}

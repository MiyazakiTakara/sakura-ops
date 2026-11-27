// Command sakuraops-server is the SakuraOps hub (P01 skeleton: health and the
// embedded web UI only).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MiyazakiTakara/sakura-ops/internal/httpserver"
	"github.com/MiyazakiTakara/sakura-ops/internal/webui"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8080", "address to listen on")
	healthcheck := flag.Bool("healthcheck", false, "probe http://<listen>/healthz and exit 0 (healthy) or 1")
	flag.Parse()

	if *healthcheck {
		ctx, cancel := context.WithTimeout(context.Background(), healthcheckTimeout)
		defer cancel()
		if err := probeHealth(ctx, *listen); err != nil {
			fmt.Fprintln(os.Stderr, "healthcheck failed:", err)
			os.Exit(1)
		}
		return
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := run(*listen, log); err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func run(addr string, log *slog.Logger) error {
	dbOpts, closeDB, err := databaseOptions(context.Background(), *databaseURLFile, log)
	if err != nil {
		return err
	}
	defer closeDB()

	srv := &http.Server{
		Addr:              addr,
		Handler:           httpserver.New(webui.FS(), dbOpts...),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", addr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

const healthcheckTimeout = 3 * time.Second

// probeHealth GETs /healthz on the address the server listens on. It exists so
// the distroless runtime image needs no curl or shell. Wildcard and empty hosts
// are probed through loopback.
func probeHealth(ctx context.Context, listen string) error {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}
	url := "http://" + net.JoinHostPort(host, port) + "/healthz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	// No redirects: /healthz answering anything but 200 is unhealthy.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

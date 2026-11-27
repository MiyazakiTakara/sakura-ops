// Command sakuraops-server is the SakuraOps hub (P01 skeleton: health and the
// embedded web UI only).
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
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
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := run(*listen, log); err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func run(addr string, log *slog.Logger) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           httpserver.New(webui.FS()),
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

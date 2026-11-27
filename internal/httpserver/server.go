// Package httpserver is the hub HTTP surface: liveness, the reserved /api
// namespace and the embedded single-page app.
package httpserver

import (
	"bytes"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/MiyazakiTakara/sakura-ops/internal/api/apitypes"
)

const immutablePrefix = "_app/immutable/"

// Option customises the handler built by New.
type Option func(*server)

// WithReadiness enables /readyz backed by r. Without it /readyz answers 503
// db_not_configured.
func WithReadiness(r Readiness) Option {
	return func(s *server) { s.ready = r }
}

// New returns the hub handler serving webFS (index.html at its root).
func New(webFS fs.FS, opts ...Option) http.Handler {
	s := &server{web: webFS, csp: contentSecurityPolicy(webFS)}
	for _, o := range opts {
		o(s)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/readyz", s.readyz)
	mux.HandleFunc("/api", s.apiNotFound)
	mux.HandleFunc("/api/", s.apiNotFound)
	mux.HandleFunc("/", s.spa)
	return s.middleware(mux)
}

type server struct {
	web   fs.FS
	csp   string
	ready Readiness
}

func (s *server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Request-ID", newRequestID())
		h.Set("Content-Security-Policy", s.csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
}

func (s *server) healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w, "GET, HEAD")
		return
	}
	writeJSON(w, http.StatusOK, apitypes.Health{Status: apitypes.HealthStatusOK})
}

// apiNotFound answers everything under /api until real endpoints exist. It must
// never fall back to index.html.
func (s *server) apiNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "No such API endpoint.")
}

func (s *server) spa(w http.ResponseWriter, r *http.Request) {
	if isReserved(r.URL.Path) {
		// e.g. /healthz/x or /API/x: reserved names never reach the SPA.
		writeError(w, http.StatusNotFound, "not_found", "Not found.")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w, "GET, HEAD")
		return
	}
	name, ok := cleanAssetPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid path.")
		return
	}

	if s.serveFile(w, r, name) {
		return
	}
	// Missing asset (static-asset extension, or lives under _app/): a real 404, so a
	// failed script/style request never receives HTML.
	if isMissingAsset(name) {
		writeError(w, http.StatusNotFound, "not_found", "Not found.")
		return
	}
	// Application route: SPA fallback.
	if !s.serveFile(w, r, "index.html") {
		writeError(w, http.StatusServiceUnavailable, "frontend_not_built",
			"The web UI is not embedded in this binary.")
	}
}

// serveFile serves name if it is a regular file in the embedded FS.
func (s *server) serveFile(w http.ResponseWriter, r *http.Request, name string) bool {
	info, err := fs.Stat(s.web, name)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	data, err := fs.ReadFile(s.web, name)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			writeError(w, http.StatusInternalServerError, "internal", "Internal error.")
			return true
		}
		return false
	}
	h := w.Header()
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
		h.Set("Content-Type", ct)
	}
	if strings.HasPrefix(name, immutablePrefix) {
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		h.Set("Cache-Control", "no-cache")
	}
	// Zero modtime: embedded files have none, so no Last-Modified is emitted.
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	return true
}

package httpserver

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/MiyazakiTakara/sakura-ops/internal/api/apitypes"
)

const indexHTML = `<!doctype html><html><body><script>boot(1)</script>INDEX</body></html>`

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	root := fstest.MapFS{
		"secret.txt":                     {Data: []byte("OUTSIDE-SECRET")},
		"dist/index.html":                {Data: []byte(indexHTML)},
		"dist/_app/immutable/app.abc.js": {Data: []byte("console.log(1)")},
		"dist/_app/version.json":         {Data: []byte(`{"version":"1"}`)},
		"dist/favicon.svg":               {Data: []byte("<svg/>")},
		"dist/.gitkeep":                  {Data: []byte("")},
		"dist/spike/terminal/inner.txt":  {Data: []byte("inner")},
	}
	sub, err := fs.Sub(root, "dist")
	if err != nil {
		t.Fatal(err)
	}
	return New(sub)
}

func do(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func decodeErr(t *testing.T, rec *httptest.ResponseRecorder) apitypes.Error {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q, want JSON", ct)
	}
	var e apitypes.Error
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}
	if e.Code == "" || e.Message == "" || e.RequestId == "" {
		t.Fatalf("incomplete error envelope: %+v", e)
	}
	return e
}

func TestHealthz(t *testing.T) {
	h := testHandler(t)
	rec := do(h, "GET", "/healthz")
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got["status"] != "ok" || len(got) != 1 {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestHealthzRejectsOtherMethods(t *testing.T) {
	h := testHandler(t)
	for _, m := range []string{"POST", "PUT", "DELETE", "PATCH"} {
		rec := do(h, m, "/healthz")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /healthz = %d, want 405", m, rec.Code)
		}
		if rec.Header().Get("Allow") == "" {
			t.Errorf("%s /healthz: missing Allow header", m)
		}
		decodeErr(t, rec)
	}
}

func TestAPINeverFallsBack(t *testing.T) {
	h := testHandler(t)
	for _, target := range []string{"/api", "/api/", "/api/unknown", "/api/v1/hosts", "/api/v1/x.js", "/API/v1/hosts", "/healthz/x", "/healthz/"} {
		for _, m := range []string{"GET", "POST"} {
			rec := do(h, m, target)
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s %s = %d, want 404", m, target, rec.Code)
				continue
			}
			if strings.Contains(rec.Body.String(), "INDEX") {
				t.Errorf("%s %s returned index.html", m, target)
			}
			decodeErr(t, rec)
		}
	}
}

func TestSPAFallback(t *testing.T) {
	h := testHandler(t)
	for _, target := range []string{"/", "/hosts", "/hosts/123/edit", "/spike/terminal", "/spike", "/tasks/linux.facts", "/tasks/agent.ping", "/hosts/lab-web-01.example"} {
		rec := do(h, "GET", target)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "INDEX") {
			t.Errorf("GET %s = %d %q, want index.html", target, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("GET %s Cache-Control = %q, want no-cache", target, got)
		}
	}
	if rec := do(testHandler(t), "HEAD", "/hosts"); rec.Code != 200 || rec.Body.Len() != 0 {
		t.Errorf("HEAD /hosts = %d with %d body bytes", rec.Code, rec.Body.Len())
	}
	if rec := do(h, "POST", "/hosts"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /hosts = %d, want 405", rec.Code)
	}
}

func TestMissingAssetsAre404NotHTML(t *testing.T) {
	h := testHandler(t)
	for _, target := range []string{"/missing.js", "/_app/immutable/nope.js", "/_app/other", "/_app/", "/_app/x", "/favicon.png", "/favicon.ico", "/x.CSS"} {
		rec := do(h, "GET", target)
		if rec.Code != 404 || strings.Contains(rec.Body.String(), "INDEX") {
			t.Errorf("GET %s = %d %q, want JSON 404", target, rec.Code, rec.Body.String())
			continue
		}
		decodeErr(t, rec)
	}
}

func TestAssetCaching(t *testing.T) {
	h := testHandler(t)
	rec := do(h, "GET", "/_app/immutable/app.abc.js")
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("immutable asset Cache-Control = %q", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("js Content-Type = %q", ct)
	}
	if got := do(h, "GET", "/_app/version.json").Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("version.json Cache-Control = %q, want no-cache", got)
	}
	if got := do(h, "GET", "/index.html").Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("index.html Cache-Control = %q, want no-cache", got)
	}
}

func TestPathTraversalCannotEscapeFS(t *testing.T) {
	h := testHandler(t)
	targets := []string{
		"/../secret.txt", "/%2e%2e/secret.txt", "/%2E%2E/secret.txt", "/..%2fsecret.txt",
		"/spike/../../secret.txt", "/spike/%2e%2e/%2e%2e/secret.txt", "/..\\secret.txt",
		"/%5c..%5csecret.txt", "/secret.txt", "//secret.txt", "/.gitkeep", "/%2egitkeep",
		"/_app/immutable/../../../secret.txt", "/%00", "/index.html%00.js",
	}
	for _, target := range targets {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.URL.Path = decodeForTest(target)
		h.ServeHTTP(rec, req)
		if strings.Contains(rec.Body.String(), "OUTSIDE-SECRET") {
			t.Errorf("%q leaked content outside the embed root", target)
		}
		// Also through the full URL parser path.
		rec = httptest.NewRecorder()
		func() {
			defer func() { _ = recover() }() // httptest.NewRequest panics on unparsable targets
			h.ServeHTTP(rec, httptest.NewRequest("GET", target, nil))
		}()
		if strings.Contains(rec.Body.String(), "OUTSIDE-SECRET") {
			t.Errorf("%q (raw target) leaked content outside the embed root", target)
		}
	}
}

func decodeForTest(s string) string {
	// Minimal percent-decoder so adversarial decoded paths reach the handler directly.
	r := strings.NewReplacer("%2e", ".", "%2E", ".", "%2f", "/", "%5c", "\\", "%00", "\x00")
	return r.Replace(s)
}

func TestSecurityHeaders(t *testing.T) {
	h := testHandler(t)
	for _, target := range []string{"/", "/hosts", "/healthz", "/api/x", "/_app/immutable/app.abc.js", "/missing.js"} {
		rec := do(h, "GET", target)
		want := map[string]string{
			"X-Content-Type-Options": "nosniff",
			"Referrer-Policy":        "no-referrer",
			"X-Frame-Options":        "DENY",
		}
		for k, v := range want {
			if got := rec.Header().Get(k); got != v {
				t.Errorf("%s %s = %q, want %q", target, k, got, v)
			}
		}
		if rec.Header().Get("X-Request-ID") == "" {
			t.Errorf("%s: missing X-Request-ID", target)
		}
		csp := rec.Header().Get("Content-Security-Policy")
		if csp == "" {
			t.Errorf("%s: missing CSP", target)
		}
	}
}

func TestCSPNoUnsafeInlineForScripts(t *testing.T) {
	csp := do(testHandler(t), "GET", "/").Header().Get("Content-Security-Policy")
	var script string
	for _, d := range strings.Split(csp, ";") {
		if d = strings.TrimSpace(d); strings.HasPrefix(d, "script-src") {
			script = d
		}
	}
	if script == "" {
		t.Fatalf("no script-src in %q", csp)
	}
	if strings.Contains(script, "unsafe-inline") || strings.Contains(script, "unsafe-eval") {
		t.Errorf("script-src allows unsafe sources: %q", script)
	}
	if !strings.Contains(script, "'sha256-") {
		t.Errorf("script-src has no hash for the inline bootstrap script: %q", script)
	}
	if !strings.Contains(csp, "frame-ancestors 'none'") || !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("CSP missing baseline directives: %q", csp)
	}
}

func TestCSPHashMatchesInlineScript(t *testing.T) {
	sum := sha256.Sum256([]byte("boot(1)"))
	want := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	csp := contentSecurityPolicy(fstest.MapFS{"index.html": {Data: []byte(indexHTML)}})
	if !strings.Contains(csp, want) {
		t.Fatalf("CSP %q lacks %s", csp, want)
	}
	// Scripts with attributes (e.g. src=) are not hashed.
	csp = contentSecurityPolicy(fstest.MapFS{"index.html": {Data: []byte(`<script src="/a.js"></script>`)}})
	if strings.Contains(csp, "sha256-") {
		t.Fatalf("external script must not be hashed: %q", csp)
	}
}

func TestFrontendNotBuilt(t *testing.T) {
	h := New(fstest.MapFS{".gitkeep": {}})
	rec := do(h, "GET", "/hosts")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	decodeErr(t, rec)
	if rec := do(h, "GET", "/healthz"); rec.Code != 200 {
		t.Fatalf("healthz must work without a build, got %d", rec.Code)
	}
}

func FuzzCleanAssetPath(f *testing.F) {
	for _, s := range []string{"/", "/a/b", "/../x", "/a/../../x", "/.git", "/a\\b", "/a\x00", "//a", "/_app/immutable/x.js", "", "a"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, p string) {
		name, ok := cleanAssetPath(p)
		if !ok {
			return
		}
		if !fs.ValidPath(name) || name == "" {
			t.Fatalf("invalid fs name %q from %q", name, p)
		}
		for _, seg := range strings.Split(name, "/") {
			if seg == ".." || seg == "." || strings.HasPrefix(seg, ".") {
				t.Fatalf("unsafe segment in %q from %q", name, p)
			}
		}
		if strings.ContainsAny(name, "\x00\\") {
			t.Fatalf("forbidden byte in %q", name)
		}
	})
}

func FuzzSPAHandlerNeverLeaks(f *testing.F) {
	for _, s := range []string{"/", "/../secret.txt", "/%2e%2e/secret.txt", "/a/../secret.txt", "/secret.txt"} {
		f.Add(s)
	}
	root := fstest.MapFS{
		"secret.txt":      {Data: []byte("OUTSIDE-SECRET")},
		"dist/index.html": {Data: []byte(indexHTML)},
	}
	sub, _ := fs.Sub(root, "dist")
	h := New(sub)
	f.Fuzz(func(t *testing.T, p string) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.URL.Path = p
		h.ServeHTTP(rec, req)
		if strings.Contains(rec.Body.String(), "OUTSIDE-SECRET") {
			t.Fatalf("path %q leaked outside content", p)
		}
		if isReserved(p) && strings.Contains(rec.Body.String(), "INDEX") {
			t.Fatalf("reserved path %q got index.html", p)
		}
	})
}

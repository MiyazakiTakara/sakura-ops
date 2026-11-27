package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/MiyazakiTakara/sakura-ops/internal/httpserver"
)

type fakeReady struct {
	v   int64
	err error
}

func (f fakeReady) Ready(context.Context) (int64, error) { return f.v, f.err }

func readyzHandler(r httpserver.Readiness) http.Handler {
	opts := []httpserver.Option{}
	if r != nil {
		opts = append(opts, httpserver.WithReadiness(r))
	}
	return httpserver.New(fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}}, opts...)
}

// responseSchema returns the JSON schema the spec declares for GET /readyz at status.
func responseSchema(t *testing.T, doc *openapi3.T, status int) *openapi3.Schema {
	t.Helper()
	op := doc.Paths.Find("/readyz").Get
	ref := op.Responses.Status(status)
	if ref == nil || ref.Value == nil {
		t.Fatalf("spec has no %d response for /readyz", status)
	}
	if ref.Value.Content.Get("application/json") == nil {
		t.Fatalf("spec %d response has no application/json content", status)
	}
	return ref.Value.Content.Get("application/json").Schema.Value
}

// Real handler output for every /readyz state must satisfy the schema the OpenAPI
// document declares for that status, and the documented 503 codes must be exactly the
// ones the handler emits.
func TestReadyzResponsesMatchSpec(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		ready    httpserver.Readiness
		method   string
		wantCode int
		wantErr  string
		wantVer  int64
	}{
		{"ready GET", fakeReady{v: 7}, "GET", 200, "", 7},
		{"not configured", nil, "GET", 503, "db_not_configured", 0},
		{"unavailable", fakeReady{err: errors.New("boom")}, "GET", 503, "db_unavailable", 0},
		{"outdated", fakeReady{v: 1, err: httpserver.ErrSchemaOutdated}, "GET", 503, "schema_outdated", 0},
		{"ahead", fakeReady{v: 9, err: httpserver.ErrSchemaAhead}, "GET", 503, "schema_ahead", 0},
		{"POST", fakeReady{v: 7}, "POST", 405, "method_not_allowed", 0},
		{"DELETE", fakeReady{v: 7}, "DELETE", 405, "method_not_allowed", 0},
	}
	documented := map[string]bool{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			readyzHandler(c.ready).ServeHTTP(rec, httptest.NewRequest(c.method, "/readyz", nil))
			if rec.Code != c.wantCode {
				t.Fatalf("status = %d, want %d: %s", rec.Code, c.wantCode, rec.Body)
			}
			var body any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body is not JSON: %v", err)
			}
			schema := func() *openapi3.Schema {
				if c.wantCode == 405 {
					r := doc.Components.Responses["MethodNotAllowed"]
					return r.Value.Content.Get("application/json").Schema.Value
				}
				return responseSchema(t, doc, c.wantCode)
			}()
			if err := schema.VisitJSON(body); err != nil {
				t.Fatalf("response violates spec: %v\n%s", err, rec.Body)
			}
			m := body.(map[string]any)
			if c.wantErr == "" {
				if m["status"] != "ready" || int64(m["schema_version"].(float64)) != c.wantVer {
					t.Fatalf("body %s", rec.Body)
				}
				return
			}
			if m["code"] != c.wantErr {
				t.Fatalf("code = %v, want %s", m["code"], c.wantErr)
			}
			if c.wantCode == 405 && rec.Header().Get("Allow") != "GET, HEAD" {
				t.Errorf("Allow = %q", rec.Header().Get("Allow"))
			}
			if c.wantCode == 503 {
				documented[c.wantErr] = true
			}
		})
	}
	desc := doc.Paths.Find("/readyz").Get.Description
	for code := range documented {
		if !strings.Contains(desc, "`"+code+"`") {
			t.Errorf("503 code %s is not documented in the /readyz description", code)
		}
	}
	if len(documented) != 4 {
		t.Errorf("exercised %d distinct 503 codes, want 4", len(documented))
	}
}

func TestReadyzHeadHasNoBody(t *testing.T) {
	rec := httptest.NewRecorder()
	readyzHandler(fakeReady{v: 1}).ServeHTTP(rec, httptest.NewRequest("HEAD", "/readyz", nil))
	if rec.Code != 200 {
		t.Fatalf("HEAD = %d", rec.Code)
	}
}

// Negative: a body that does not match the declared Readiness schema is rejected.
func TestReadinessSchemaRejectsBadBodies(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	s := doc.Components.Schemas["Readiness"].Value
	for name, v := range map[string]any{
		"wrong status":    map[string]any{"status": "ok", "schema_version": 1},
		"missing version": map[string]any{"status": "ready"},
		"extra field":     map[string]any{"status": "ready", "schema_version": 1, "x": 1},
		"string version":  map[string]any{"status": "ready", "schema_version": "1"},
	} {
		if s.VisitJSON(v) == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

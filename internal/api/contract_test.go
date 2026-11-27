// Package api holds cross-language contract tests: the shared fixtures in api/fixtures
// are validated against the OpenAPI schemas here and, with the same expectations, by the
// Vitest tests in frontend/src/lib/api.
package api_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/MiyazakiTakara/sakura-ops/internal/api/apitypes"
)

const (
	specPath    = "../../api/openapi/sakuraops.v1.yaml"
	fixturesDir = "../../api/fixtures"
)

// schemaFor maps a fixture file prefix to its component schema.
var schemaFor = map[string]string{"error": "Error", "health": "Health"}

func loadSchemas(t *testing.T) map[string]*openapi3.Schema {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("load OpenAPI document: %v", err)
	}
	out := map[string]*openapi3.Schema{}
	for prefix, name := range schemaFor {
		ref := doc.Components.Schemas[name]
		if ref == nil || ref.Value == nil {
			t.Fatalf("schema %s missing", name)
		}
		out[prefix] = ref.Value
	}
	return out
}

// Fixture names are <schema>.<valid|invalid>.<case>.json; the middle part is the
// expectation shared with the TypeScript tests.
func TestFixturesMatchSchema(t *testing.T) {
	schemas := loadSchemas(t)
	files, err := filepath.Glob(filepath.Join(fixturesDir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures found: %v", err)
	}
	valid, invalid := 0, 0
	for _, f := range files {
		name := filepath.Base(f)
		parts := strings.Split(name, ".")
		if len(parts) < 4 || (parts[1] != "valid" && parts[1] != "invalid") {
			t.Fatalf("fixture %s does not follow <schema>.<valid|invalid>.<case>.json", name)
		}
		schema, ok := schemas[parts[0]]
		if !ok {
			t.Fatalf("fixture %s: unknown schema prefix", name)
		}
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			var v any
			if err := json.Unmarshal(raw, &v); err != nil {
				t.Fatalf("fixture is not JSON: %v", err)
			}
			err = schema.VisitJSON(v)
			if parts[1] == "valid" && err != nil {
				t.Errorf("valid fixture rejected: %v", err)
			}
			if parts[1] == "invalid" && err == nil {
				t.Errorf("invalid fixture accepted")
			}
		})
		if parts[1] == "valid" {
			valid++
		} else {
			invalid++
		}
	}
	if valid == 0 || invalid == 0 {
		t.Fatalf("need both valid and invalid fixtures, have %d and %d", valid, invalid)
	}
}

// The generated Go types must decode every valid error fixture and re-encode it without
// losing data, so server output and fixtures cannot drift apart.
func TestValidErrorFixturesRoundTripThroughGeneratedType(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(fixturesDir, "error.valid.*.json"))
	if len(files) == 0 {
		t.Fatal("no valid error fixtures")
	}
	for _, f := range files {
		raw, _ := os.ReadFile(f)
		var e apitypes.Error
		if err := json.Unmarshal(raw, &e); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		again, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		var a, b any
		_ = json.Unmarshal(raw, &a)
		_ = json.Unmarshal(again, &b)
		x, _ := json.Marshal(a)
		y, _ := json.Marshal(b)
		if string(x) != string(y) {
			t.Errorf("%s: round trip changed JSON:\n%s\n%s", filepath.Base(f), x, y)
		}
	}
}

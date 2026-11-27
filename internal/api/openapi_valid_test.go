package api_test

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// The committed document must be a valid OpenAPI 3 document with every $ref resolvable.
func TestOpenAPIDocumentIsValid(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("load OpenAPI document: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("OpenAPI document is invalid: %v", err)
	}
	if doc.Paths == nil || doc.Paths.Len() == 0 {
		t.Fatal("OpenAPI document defines no paths")
	}
}

// Negative case: validation must reject a document that references a missing schema.
func TestOpenAPIValidationRejectsBrokenDocument(t *testing.T) {
	const broken = `openapi: 3.0.3
info: {title: t, version: "1"}
paths:
  /x:
    get:
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: {$ref: "#/components/schemas/Missing"}
`
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(broken))
	if err == nil {
		err = doc.Validate(context.Background())
	}
	if err == nil {
		t.Fatal("a dangling $ref must fail load or validation")
	}
}

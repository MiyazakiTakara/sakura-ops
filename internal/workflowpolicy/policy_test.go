package workflowpolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sha = "3d3c42e5aac5ba805825da76410c181273ba90b1"

const good = `
name: ok
on:
  pull_request:
    branches: [main]
permissions:
  contents: read
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@` + sha + ` # v7
        with:
          persist-credentials: false
      - run: echo hi
`

func TestRepositoryWorkflowsFollowPolicy(t *testing.T) {
	files, err := filepath.Glob("../../.github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	more, _ := filepath.Glob("../../.github/workflows/*.yaml")
	files = append(files, more...)
	if len(files) == 0 {
		t.Fatal("no workflow files found")
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, viol := range Check(data) {
			t.Errorf("%s: %s", filepath.Base(f), viol)
		}
	}
}

func TestGoodWorkflowPasses(t *testing.T) {
	if v := Check([]byte(good)); len(v) != 0 {
		t.Fatalf("unexpected violations: %v", v)
	}
}

func TestNegativeCases(t *testing.T) {
	cases := []struct {
		name, from, to, want string
	}{
		{"push trigger", "pull_request:\n    branches: [main]", "push:\n    branches: [main]", "only pull_request"},
		{"pull_request_target", "pull_request:", "pull_request_target:", "pull_request_target"},
		{"schedule added", "permissions:", "  schedule:\n    - cron: '0 0 * * *'\npermissions:", "schedule"},
		{"workflow_dispatch added", "permissions:", "  workflow_dispatch:\npermissions:", "workflow_dispatch"},
		{"no top permissions", "permissions:\n  contents: read\n", "", "permissions` missing"},
		{"top write", "contents: read", "contents: write", "contents: read"},
		{"top extra write", "contents: read", "contents: read\n  id-token: write", "broader than read"},
		{"job write", "    runs-on: ubuntu-latest", "    permissions:\n      contents: write\n    runs-on: ubuntu-latest", "broader than read"},
		{"job write-all", "    runs-on: ubuntu-latest", "    permissions: write-all\n    runs-on: ubuntu-latest", "explicit read/none map"},
		{"tag pin", sha + " # v7", "v4", "not pinned"},
		{"short sha", sha + " # v7", sha[:7], "not pinned"},
		{"persist true", "persist-credentials: false", "persist-credentials: true", "persist-credentials"},
		{"persist missing", "        with:\n          persist-credentials: false\n", "", "persist-credentials"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(good, c.from) {
				t.Fatalf("test bug: %q not in template", c.from)
			}
			doc := strings.Replace(good, c.from, c.to, 1)
			v := Check([]byte(doc))
			if len(v) == 0 {
				t.Fatalf("expected a violation for %s", c.name)
			}
			if !strings.Contains(strings.Join(v, "\n"), c.want) {
				t.Fatalf("violations %v lack %q", v, c.want)
			}
		})
	}
}

func TestInvalidYAMLFails(t *testing.T) {
	if v := Check([]byte("on: [unterminated")); len(v) == 0 {
		t.Fatal("invalid YAML must be a violation")
	}
}

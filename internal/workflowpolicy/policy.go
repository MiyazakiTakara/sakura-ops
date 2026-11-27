// Package workflowpolicy enforces the repository policy for GitHub Actions workflows
// (docs/dev/ci.md): pull_request only, read-only token, SHA-pinned actions and checkout
// without persisted credentials. It is used by tests only; it never touches the network.
package workflowpolicy

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"
)

var shaPin = regexp.MustCompile(`^[^@\s]+@[0-9a-f]{40}$`)

// Check returns every policy violation found in one workflow document. An unparsable
// document is itself a violation.
func Check(data []byte) []string {
	var wf map[string]any
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return []string{fmt.Sprintf("not valid YAML: %v", err)}
	}
	var v []string
	v = append(v, checkTriggers(wf)...)
	v = append(v, checkTopPermissions(wf)...)
	jobs, _ := wf["jobs"].(map[string]any)
	if len(jobs) == 0 {
		v = append(v, "no jobs defined")
	}
	names := make([]string, 0, len(jobs))
	for n := range jobs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		job, _ := jobs[n].(map[string]any)
		v = append(v, checkJob(n, job)...)
	}
	return v
}

func checkTriggers(wf map[string]any) []string {
	// YAML 1.1 parsers may turn the key `on` into a boolean.
	on, ok := wf["on"]
	if !ok {
		on, ok = wf["true"]
	}
	if !ok {
		return []string{"no `on` trigger section"}
	}
	var events []string
	switch t := on.(type) {
	case string:
		events = []string{t}
	case []any:
		for _, e := range t {
			events = append(events, fmt.Sprint(e))
		}
	case map[string]any:
		for e := range t {
			events = append(events, e)
		}
	default:
		return []string{fmt.Sprintf("unsupported `on` form %T", on)}
	}
	var v []string
	for _, e := range events {
		if e != "pull_request" {
			v = append(v, fmt.Sprintf("trigger %q is not allowed; only pull_request", e))
		}
	}
	if len(events) == 0 {
		v = append(v, "no trigger configured")
	}
	return v
}

func checkTopPermissions(wf map[string]any) []string {
	p, ok := wf["permissions"]
	if !ok {
		return []string{"top-level `permissions` missing; require `contents: read`"}
	}
	m, ok := p.(map[string]any)
	if !ok {
		return []string{fmt.Sprintf("top-level permissions must be a map with contents: read, got %v", p)}
	}
	var v []string
	if fmt.Sprint(m["contents"]) != "read" {
		v = append(v, "top-level permissions must set `contents: read`")
	}
	for k, val := range m {
		if s := fmt.Sprint(val); s != "read" && s != "none" {
			v = append(v, fmt.Sprintf("top-level permission %s: %s is broader than read", k, s))
		}
	}
	return v
}

func checkJob(name string, job map[string]any) []string {
	var v []string
	if p, ok := job["permissions"]; ok {
		switch pm := p.(type) {
		case map[string]any:
			for k, val := range pm {
				if s := fmt.Sprint(val); s != "read" && s != "none" {
					v = append(v, fmt.Sprintf("job %s: permission %s: %s is broader than read", name, k, s))
				}
			}
		default:
			v = append(v, fmt.Sprintf("job %s: permissions must be an explicit read/none map, got %v", name, p))
		}
	}
	if u, ok := job["uses"].(string); ok {
		v = append(v, checkUses(name, u, nil)...)
	}
	steps, _ := job["steps"].([]any)
	for i, s := range steps {
		step, _ := s.(map[string]any)
		u, ok := step["uses"].(string)
		if !ok {
			continue
		}
		with, _ := step["with"].(map[string]any)
		v = append(v, checkUses(fmt.Sprintf("%s step %d", name, i+1), u, with)...)
	}
	return v
}

func checkUses(where, uses string, with map[string]any) []string {
	var v []string
	if !strings.HasPrefix(uses, "./") && !shaPin.MatchString(uses) {
		v = append(v, fmt.Sprintf("job %s: %q is not pinned to a 40-hex commit SHA", where, uses))
	}
	if strings.HasPrefix(uses, "actions/checkout@") {
		if pc, ok := with["persist-credentials"].(bool); !ok || pc {
			v = append(v, fmt.Sprintf("job %s: actions/checkout needs `persist-credentials: false`", where))
		}
	}
	return v
}

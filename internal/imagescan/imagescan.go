// Package imagescan evaluates grype JSON reports against the reviewed ignore
// file. Policy: HIGH or CRITICAL findings with a fix available fail; unfixed
// HIGH or CRITICAL findings are warnings. Ignore entries need an id, image,
// package, reason, reviewer and an unexpired date; unused and expired entries
// fail. Nothing here lowers severities or short-circuits a failure.
package imagescan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

// IgnoreEntry is one reviewed exception for exactly one finding.
type IgnoreEntry struct {
	ID       string `json:"id"`
	Image    string `json:"image"`
	Package  string `json:"package"`
	Reason   string `json:"reason"`
	Reviewer string `json:"reviewer"`
	Expires  string `json:"expires"`

	expires time.Time
}

// IgnoreFile is the on-disk format of supply-chain/image-scan-ignore.json.
type IgnoreFile struct {
	Ignores []IgnoreEntry `json:"ignores"`
}

// LoadIgnore reads and validates the ignore file. Unknown fields are rejected.
func LoadIgnore(path string) (*IgnoreFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ignore file: %w", err)
	}
	return ParseIgnore(b)
}

// ParseIgnore validates ignore file content.
func ParseIgnore(b []byte) (*IgnoreFile, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var f IgnoreFile
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("parse ignore file: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("parse ignore file: trailing data")
	}
	seen := map[string]bool{}
	for i := range f.Ignores {
		e := &f.Ignores[i]
		for name, v := range map[string]string{"id": e.ID, "image": e.Image, "package": e.Package,
			"reason": e.Reason, "reviewer": e.Reviewer, "expires": e.Expires} {
			if strings.TrimSpace(v) == "" {
				return nil, fmt.Errorf("ignore entry %d: missing %s", i, name)
			}
			if strings.ContainsAny(v, "*?") && name != "reason" {
				return nil, fmt.Errorf("ignore entry %d: wildcards are not allowed in %s", i, name)
			}
		}
		t, err := time.Parse(dateLayout, e.Expires)
		if err != nil {
			return nil, fmt.Errorf("ignore entry %d: expires must be YYYY-MM-DD: %w", i, err)
		}
		e.expires = t
		key := e.ID + "|" + e.Image + "|" + e.Package
		if seen[key] {
			return nil, fmt.Errorf("ignore entry %d: duplicate %s", i, key)
		}
		seen[key] = true
	}
	return &f, nil
}

// Finding is one HIGH or CRITICAL match.
type Finding struct {
	Image    string
	ID       string
	Related  []string
	Severity string
	Package  string
	Version  string
	Fixed    []string
}

type grypeReport struct {
	Matches []struct {
		Vulnerability struct {
			ID       string `json:"id"`
			Severity string `json:"severity"`
			Fix      struct {
				Versions []string `json:"versions"`
				State    string   `json:"state"`
			} `json:"fix"`
		} `json:"vulnerability"`
		Related []struct {
			ID string `json:"id"`
		} `json:"relatedVulnerabilities"`
		Artifact struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"artifact"`
	} `json:"matches"`
	Source *struct{} `json:"source"`
}

// ParseReport extracts HIGH and CRITICAL findings from a grype JSON report.
// A report without a "source" object is rejected so that an empty or wrong
// file cannot pass as a clean scan.
func ParseReport(image string, b []byte) ([]Finding, error) {
	var r grypeReport
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("%s: parse grype report: %w", image, err)
	}
	if r.Source == nil {
		return nil, fmt.Errorf("%s: not a grype report (no source)", image)
	}
	var out []Finding
	for _, m := range r.Matches {
		sev := strings.ToUpper(m.Vulnerability.Severity)
		if sev != "HIGH" && sev != "CRITICAL" {
			continue
		}
		f := Finding{Image: image, ID: m.Vulnerability.ID, Severity: sev,
			Package: m.Artifact.Name, Version: m.Artifact.Version}
		for _, rel := range m.Related {
			f.Related = append(f.Related, rel.ID)
		}
		f.Fixed = m.Vulnerability.Fix.Versions
		if len(f.Fixed) == 0 && strings.EqualFold(m.Vulnerability.Fix.State, "fixed") {
			f.Fixed = []string{"(fixed, version not listed)"}
		}
		out = append(out, f)
	}
	return out, nil
}

// Result is the outcome of Evaluate.
type Result struct {
	Failures   []Finding // fixable HIGH/CRITICAL, not ignored
	Warnings   []Finding // unfixed HIGH/CRITICAL, not ignored
	Suppressed []Finding
	Problems   []string // expired or unused ignore entries
}

// OK reports whether the scan passes.
func (r Result) OK() bool { return len(r.Failures) == 0 && len(r.Problems) == 0 }

func (e IgnoreEntry) matches(f Finding) bool {
	if e.Image != f.Image || e.Package != f.Package {
		return false
	}
	if e.ID == f.ID {
		return true
	}
	for _, r := range f.Related {
		if e.ID == r {
			return true
		}
	}
	return false
}

// Evaluate applies the ignore file. An entry is expired when now is after the
// end of its expiry day, and unused when it matches no finding.
func Evaluate(findings []Finding, ign *IgnoreFile, now time.Time) Result {
	var res Result
	used := make([]bool, len(ign.Ignores))
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	for _, e := range ign.Ignores {
		if today.After(e.expires) {
			res.Problems = append(res.Problems, fmt.Sprintf("expired ignore entry: %s %s %s (expired %s, reviewer %s)",
				e.ID, e.Image, e.Package, e.Expires, e.Reviewer))
		}
	}
	for _, f := range findings {
		ignored := false
		for i, e := range ign.Ignores {
			if e.matches(f) && !today.After(e.expires) {
				used[i], ignored = true, true
			} else if e.matches(f) {
				used[i] = true // expired: already reported; not reported again as unused
			}
		}
		switch {
		case ignored:
			res.Suppressed = append(res.Suppressed, f)
		case len(f.Fixed) > 0:
			res.Failures = append(res.Failures, f)
		default:
			res.Warnings = append(res.Warnings, f)
		}
	}
	for i, e := range ign.Ignores {
		if !used[i] {
			res.Problems = append(res.Problems, fmt.Sprintf("unused ignore entry: %s %s %s (matches no HIGH/CRITICAL finding; remove it)",
				e.ID, e.Image, e.Package))
		}
	}
	for _, s := range [][]Finding{res.Failures, res.Warnings, res.Suppressed} {
		sort.SliceStable(s, func(a, b int) bool {
			if s[a].Image != s[b].Image {
				return s[a].Image < s[b].Image
			}
			return s[a].ID < s[b].ID
		})
	}
	return res
}

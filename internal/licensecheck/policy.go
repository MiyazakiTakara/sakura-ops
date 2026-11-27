// Package licensecheck evaluates CycloneDX SBOMs against the SakuraOps licence
// policy (supply-chain/license-policy.json). It uses only the standard library.
package licensecheck

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Allowed is one allowlisted SPDX identifier.
type Allowed struct {
	ID   string `json:"id"`
	Note string `json:"note,omitempty"`
}

// Exception is a reviewed waiver of one exact package URL (type, name and
// version) for one exact licence expression. It never waives anything else:
// a different version, ecosystem or licence is still a finding, and a
// component without a purl can never be waived.
type Exception struct {
	PURL     string `json:"purl"`
	Licence  string `json:"license"`
	Reason   string `json:"reason"`
	Reviewer string `json:"reviewer"`
}

// Policy is the decoded policy file.
type Policy struct {
	Version        int         `json:"version"`
	Allowed        []Allowed   `json:"allowed"`
	DeniedIDs      []string    `json:"denied_ids"`
	DeniedPrefixes []string    `json:"denied_prefixes"`
	FirstParty     []string    `json:"first_party"`
	Exceptions     []Exception `json:"exceptions"`

	allowed    map[string]bool
	denied     map[string]bool
	firstParty map[string]bool // purl without version
	exceptions map[string]Exception
}

// LoadPolicy reads and validates a policy file.
func LoadPolicy(path string) (*Policy, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open policy: %w", err)
	}
	defer f.Close()
	return ParsePolicy(f)
}

// ParsePolicy decodes and validates a policy. Unknown fields are rejected.
func ParsePolicy(r io.Reader) (*Policy, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var p Policy
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("decode policy: %w", err)
	}
	if p.Version != 1 {
		return nil, fmt.Errorf("unsupported policy version %d", p.Version)
	}
	p.allowed = map[string]bool{}
	for _, a := range p.Allowed {
		if strings.TrimSpace(a.ID) == "" {
			return nil, errors.New("policy: empty allowed id")
		}
		p.allowed[strings.ToLower(a.ID)] = true
	}
	p.denied = map[string]bool{}
	for _, d := range p.DeniedIDs {
		p.denied[strings.ToLower(d)] = true
	}
	for id := range p.allowed {
		if p.denied[id] || p.deniedByPrefix(id) {
			return nil, fmt.Errorf("policy: %q is both allowed and denied", id)
		}
	}
	p.firstParty = map[string]bool{}
	for _, n := range p.FirstParty {
		n = strings.TrimSpace(n)
		if !strings.HasPrefix(n, "pkg:") || len(n) == len("pkg:") || strings.ContainsAny(n, "@?#") {
			return nil, fmt.Errorf("policy: first_party %q must be a versionless package URL such as pkg:npm/name", n)
		}
		p.firstParty[n] = true
	}
	p.exceptions = map[string]Exception{}
	for _, e := range p.Exceptions {
		e.PURL = strings.TrimSpace(e.PURL)
		e.Licence = normLicence(e.Licence)
		if e.PURL == "" || e.Licence == "" || strings.TrimSpace(e.Reason) == "" || strings.TrimSpace(e.Reviewer) == "" {
			return nil, fmt.Errorf("policy: exception %q needs purl, license, reason and reviewer", e.PURL)
		}
		if !purlHasVersion(e.PURL) {
			return nil, fmt.Errorf("policy: exception purl %q must be a full package URL with type, name and version", e.PURL)
		}
		if _, dup := p.exceptions[e.PURL]; dup {
			return nil, fmt.Errorf("policy: duplicate exception for %q", e.PURL)
		}
		p.exceptions[e.PURL] = e
	}
	return &p, nil
}

func (p *Policy) deniedByPrefix(id string) bool {
	for _, pre := range p.DeniedPrefixes {
		if strings.HasPrefix(id, strings.ToLower(pre)) {
			return true
		}
	}
	return false
}

// normLicence is the normalization applied to both sides of an exception
// licence comparison: surrounding whitespace is trimmed and inner whitespace
// runs collapse to one space. Comparison is otherwise exact and case-sensitive.
func normLicence(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// purlHasVersion reports whether s looks like pkg:type/name@version with a
// non-empty type, name and version. Qualifiers and subpath are not parsed; the
// whole string is compared verbatim.
func purlHasVersion(s string) bool {
	rest, ok := strings.CutPrefix(s, "pkg:")
	if !ok {
		return false
	}
	if i := strings.IndexAny(rest, "?#"); i >= 0 {
		rest = rest[:i]
	}
	at := strings.LastIndex(rest, "@")
	if at <= 0 || at == len(rest)-1 {
		return false
	}
	typ, name, ok := strings.Cut(rest[:at], "/")
	return ok && typ != "" && name != ""
}

// purlBase returns the purl without version, qualifiers and subpath, or "" if
// s is not a package URL.
func purlBase(s string) string {
	rest, ok := strings.CutPrefix(strings.TrimSpace(s), "pkg:")
	if !ok {
		return ""
	}
	if i := strings.IndexAny(rest, "@?#"); i >= 0 {
		rest = rest[:i]
	}
	return "pkg:" + rest
}

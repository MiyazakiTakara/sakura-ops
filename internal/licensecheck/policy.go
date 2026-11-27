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

// Exception is a reviewed, exact package@version waiver.
type Exception struct {
	Package  string `json:"package"`
	Version  string `json:"version"`
	Licence  string `json:"license,omitempty"`
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
	firstParty map[string]bool
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
		p.firstParty[n] = true
	}
	p.exceptions = map[string]Exception{}
	for _, e := range p.Exceptions {
		if e.Package == "" || e.Version == "" || e.Reason == "" || e.Reviewer == "" {
			return nil, fmt.Errorf("policy: exception %q needs package, version, reason and reviewer", e.Package)
		}
		p.exceptions[e.Package+"@"+e.Version] = e
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

package licensecheck

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type bom struct {
	BOMFormat string `json:"bomFormat"`
	Metadata  struct {
		// The root component is not a dependency and is never counted, but
		// components nested under it are.
		Component *component `json:"component"`
	} `json:"metadata"`
	Components []component `json:"components"`
}

// maxDepth bounds nested components so a hostile document cannot recurse deeply.
const maxDepth = 32

type component struct {
	Type     string         `json:"type"`
	Name     string         `json:"name"`
	Version  string         `json:"version"`
	PURL     string         `json:"purl"`
	Licenses []licenseEntry `json:"licenses"`

	Components []component `json:"components"`
}

type licenseEntry struct {
	License *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"license"`
	Expression string `json:"expression"`
}

// Finding is one component that failed the policy.
type Finding struct {
	SBOM    string
	Package string
	Version string
	PURL    string
	Licence string
	Reason  string
}

// Result of checking one or more SBOMs.
type Result struct {
	Checked   int            // components evaluated
	Counts    map[string]int // licence string -> components (all SBOMs, duplicates counted once per SBOM)
	Findings  []Finding
	Excepted  []string // purls waived by an exception
	Unusedexc []string // exception purls that waived nothing; a failure
}

// OK reports whether the run passes: no findings and no stale exceptions.
func (r *Result) OK() bool { return len(r.Findings) == 0 && len(r.Unusedexc) == 0 }

// Check reads CycloneDX JSON files and evaluates every library component.
func (p *Policy) Check(paths []string) (*Result, error) {
	res := &Result{Counts: map[string]int{}}
	used := map[string]bool{}
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open sbom: %w", err)
		}
		err = p.checkOne(f, path, res, used)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	for k := range p.exceptions {
		if !used[k] {
			res.Unusedexc = append(res.Unusedexc, k)
		}
	}
	sort.Strings(res.Unusedexc)
	sort.Strings(res.Excepted)
	return res, nil
}

func (p *Policy) checkOne(r io.Reader, name string, res *Result, used map[string]bool) error {
	var b bom
	if err := json.NewDecoder(r).Decode(&b); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if b.BOMFormat != "CycloneDX" {
		return fmt.Errorf("not a CycloneDX document (bomFormat=%q)", b.BOMFormat)
	}
	before := res.Checked
	if b.Metadata.Component != nil {
		if err := p.walk(b.Metadata.Component.Components, 1, name, res, used); err != nil {
			return err
		}
	}
	if err := p.walk(b.Components, 1, name, res, used); err != nil {
		return err
	}
	if res.Checked == before {
		return fmt.Errorf("SBOM yielded zero evaluated components (empty, or only file and first-party entries); refusing to pass an empty scan")
	}
	return nil
}

// isFirstParty matches by package URL only: the purl without version must
// equal a first_party entry, so the ecosystem is part of the identity and a
// third-party package that merely shares a name cannot be skipped. Components
// without a purl are never first-party.
func (p *Policy) isFirstParty(c component) bool {
	base := purlBase(c.PURL)
	return base != "" && p.firstParty[base]
}

// walk evaluates every component and all components nested under it. File
// entries and first-party entries are skipped themselves, but their children
// are still evaluated so nothing can be hidden under them.
func (p *Policy) walk(cs []component, depth int, name string, res *Result, used map[string]bool) error {
	if depth > maxDepth {
		return fmt.Errorf("components nested deeper than %d levels", maxDepth)
	}
	for _, c := range cs {
		if c.Type != "file" && !p.isFirstParty(c) {
			p.evaluate(c, name, res, used)
		}
		if err := p.walk(c.Components, depth+1, name, res, used); err != nil {
			return err
		}
	}
	return nil
}

func (p *Policy) evaluate(c component, name string, res *Result, used map[string]bool) {
	res.Checked++
	lic, expr := licenceString(c)
	res.Counts[lic]++
	var v Verdict
	var reason string
	if expr == "" {
		v, reason = Unlisted, "no licence declared (unknown)"
	} else if ev, err := p.EvalExpression(expr); err != nil {
		v, reason = Unlisted, "unparseable licence: "+err.Error()
	} else {
		v, reason = ev, ev.String()
	}
	if v == Allow {
		return
	}
	purl := strings.TrimSpace(c.PURL)
	if e, ok := p.exceptions[purl]; ok && purl != "" && e.Licence == normLicence(lic) {
		used[purl] = true
		res.Excepted = append(res.Excepted, purl)
		return
	}
	res.Findings = append(res.Findings, Finding{name, c.Name, c.Version, purl, lic, reason})
}

// licenceString renders the declared licences and the expression that is
// evaluated. Several separate entries all apply, so they are joined with AND.
func licenceString(c component) (display, expr string) {
	var parts []string
	for _, l := range c.Licenses {
		switch {
		case l.Expression != "":
			parts = append(parts, "("+l.Expression+")")
		case l.License != nil && l.License.ID != "":
			parts = append(parts, l.License.ID)
		case l.License != nil && l.License.Name != "":
			// A free-text name is not an SPDX identifier; it can only match
			// the allowlist verbatim, which is intended.
			parts = append(parts, strings.ReplaceAll(l.License.Name, " ", "-"))
		}
	}
	if len(parts) == 0 {
		return "UNKNOWN", ""
	}
	expr = strings.Join(parts, " AND ")
	return expr, expr
}

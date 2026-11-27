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
	BOMFormat  string      `json:"bomFormat"`
	Components []component `json:"components"`
}

type component struct {
	Type     string         `json:"type"`
	Name     string         `json:"name"`
	Version  string         `json:"version"`
	Licenses []licenseEntry `json:"licenses"`
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
	Licence string
	Reason  string
}

// Result of checking one or more SBOMs.
type Result struct {
	Checked   int            // components evaluated
	Counts    map[string]int // licence string -> components (all SBOMs, duplicates counted once per SBOM)
	Findings  []Finding
	Excepted  []string // package@version waived by an exception
	Unusedexc []string // exceptions in the policy that matched nothing
}

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
	if len(b.Components) == 0 {
		return fmt.Errorf("SBOM has no components; refusing to pass an empty scan")
	}
	for _, c := range b.Components {
		if c.Type == "file" || p.firstParty[c.Name] {
			continue
		}
		res.Checked++
		lic, expr := licenceString(c)
		res.Counts[lic]++
		key := c.Name + "@" + c.Version
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
			continue
		}
		if _, ok := p.exceptions[key]; ok {
			used[key] = true
			res.Excepted = append(res.Excepted, key)
			continue
		}
		res.Findings = append(res.Findings, Finding{name, c.Name, c.Version, lic, reason})
	}
	return nil
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

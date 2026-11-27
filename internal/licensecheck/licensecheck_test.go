package licensecheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPolicy = `{
  "version": 1,
  "allowed": [{"id":"MIT"},{"id":"Apache-2.0"},{"id":"BSD-3-Clause"},{"id":"MPL-2.0"},
              {"id":"Apache-2.0 WITH LLVM-exception"}],
  "denied_ids": ["SSPL-1.0","NOASSERTION"],
  "denied_prefixes": ["GPL-","AGPL-","LGPL-"],
  "first_party": ["pkg:golang/example.test/self","pkg:npm/self"],
  "exceptions": [{"purl":"pkg:npm/waived@1.0.0","license":"Zlib","reason":"fixture","reviewer":"tester"}]
}`

func mustPolicy(t *testing.T) *Policy {
	t.Helper()
	p, err := ParsePolicy(strings.NewReader(testPolicy))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEvalExpression(t *testing.T) {
	p := mustPolicy(t)
	cases := []struct {
		expr string
		want Verdict
	}{
		{"MIT", Allow},
		{"mit", Allow},
		{"GPL-3.0-only", Denied},
		{"GPL-2.0+", Denied},
		{"LGPL-2.1-or-later", Denied},
		{"SSPL-1.0", Denied},
		{"NOASSERTION", Denied},
		{"Zlib", Unlisted},
		{"MIT OR Apache-2.0", Allow},
		{"GPL-3.0-only OR MIT", Allow},
		{"GPL-3.0-only OR Zlib", Denied},
		{"Zlib OR Foo", Unlisted},
		{"MIT AND Apache-2.0", Allow},
		{"MIT AND GPL-3.0-only", Denied},
		{"MIT AND Zlib", Unlisted},
		{"(MIT OR Zlib) AND BSD-3-Clause", Allow},
		{"MIT OR Zlib AND GPL-3.0-only", Allow}, // AND binds tighter than OR
		{"(Zlib OR Foo) AND MIT", Unlisted},
		{"Apache-2.0 WITH LLVM-exception", Allow},
		{"Apache-2.0 WITH Other-exception", Unlisted},
		{"GPL-2.0-only WITH Classpath-exception-2.0", Denied},
	}
	for _, c := range cases {
		got, err := p.EvalExpression(c.expr)
		if err != nil {
			t.Errorf("%q: %v", c.expr, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q = %v, want %v", c.expr, got, c.want)
		}
	}
}

func TestEvalExpressionMalformed(t *testing.T) {
	p := mustPolicy(t)
	for _, e := range []string{"", "  ", "MIT OR", "AND MIT", "(MIT", "MIT)", "MIT MIT", "MIT WITH", "OR"} {
		if v, err := p.EvalExpression(e); err == nil {
			t.Errorf("%q: expected error, got %v", e, v)
		}
	}
}

func TestParsePolicyRejects(t *testing.T) {
	bad := map[string]string{
		"version":           `{"version":2}`,
		"unknown":           `{"version":1,"extra":1}`,
		"both":              `{"version":1,"allowed":[{"id":"GPL-3.0-only"}],"denied_prefixes":["GPL-"]}`,
		"exc no why":        `{"version":1,"exceptions":[{"purl":"pkg:npm/a@1","license":"Zlib","reviewer":"x"}]}`,
		"exc no review":     `{"version":1,"exceptions":[{"purl":"pkg:npm/a@1","license":"Zlib","reason":"x"}]}`,
		"exc no purl":       `{"version":1,"exceptions":[{"license":"Zlib","reason":"x","reviewer":"y"}]}`,
		"exc no licence":    `{"version":1,"exceptions":[{"purl":"pkg:npm/a@1","reason":"x","reviewer":"y"}]}`,
		"exc versionless":   `{"version":1,"exceptions":[{"purl":"pkg:npm/a","license":"Zlib","reason":"x","reviewer":"y"}]}`,
		"exc empty version": `{"version":1,"exceptions":[{"purl":"pkg:npm/a@","license":"Zlib","reason":"x","reviewer":"y"}]}`,
		"exc not purl":      `{"version":1,"exceptions":[{"purl":"a@1","license":"Zlib","reason":"x","reviewer":"y"}]}`,
		"exc old fields":    `{"version":1,"exceptions":[{"package":"a","version":"1","purl":"pkg:npm/a@1","license":"Zlib","reason":"x","reviewer":"y"}]}`,
		"exc duplicate": `{"version":1,"exceptions":[` +
			`{"purl":"pkg:npm/a@1","license":"Zlib","reason":"x","reviewer":"y"},` +
			`{"purl":"pkg:npm/a@1","license":"MIT","reason":"x","reviewer":"y"}]}`,
		"first party name":      `{"version":1,"first_party":["sakuraops-frontend"]}`,
		"first party versioned": `{"version":1,"first_party":["pkg:npm/self@1"]}`,
	}
	for n, s := range bad {
		if _, err := ParsePolicy(strings.NewReader(s)); err == nil {
			t.Errorf("%s: expected error", n)
		}
	}
}

func writeSBOM(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "x.cdx.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const sbomFixture = `{"bomFormat":"CycloneDX","components":[
 {"type":"library","name":"ok","version":"1","licenses":[{"license":{"id":"MIT"}}]},
 {"type":"library","name":"dual","version":"1","licenses":[{"expression":"GPL-3.0-only OR Apache-2.0"}]},
 {"type":"library","name":"bad","version":"2","licenses":[{"license":{"id":"AGPL-3.0-only"}}]},
 {"type":"library","name":"none","version":"3"},
 {"type":"library","name":"named","version":"4","licenses":[{"license":{"name":"Custom Terms"}}]},
 {"type":"library","name":"both","version":"5","licenses":[{"license":{"id":"MIT"}},{"license":{"id":"Zlib"}}]},
 {"type":"library","name":"waived","version":"1.0.0","purl":"pkg:npm/waived@1.0.0","licenses":[{"license":{"id":"Zlib"}}]},
 {"type":"library","name":"waived","version":"1.0.1","purl":"pkg:npm/waived@1.0.1","licenses":[{"license":{"id":"Zlib"}}]},
 {"type":"library","name":"example.test/self","version":"UNKNOWN","purl":"pkg:golang/example.test/self"},
 {"type":"file","name":"/package.json"}
]}`

func TestCheck(t *testing.T) {
	p := mustPolicy(t)
	res, err := p.Check([]string{writeSBOM(t, sbomFixture)})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range res.Findings {
		got[f.Package+"@"+f.Version] = f.Reason
	}
	want := map[string]string{
		"bad@2":        "denied",
		"none@3":       "no licence declared (unknown)",
		"named@4":      "not on the allowlist",
		"both@5":       "not on the allowlist",
		"waived@1.0.1": "not on the allowlist",
	}
	if len(got) != len(want) {
		t.Fatalf("findings = %v, want %v", got, want)
	}
	for k, r := range want {
		if got[k] != r {
			t.Errorf("%s: %q, want %q", k, got[k], r)
		}
	}
	if len(res.Excepted) != 1 || res.Excepted[0] != "pkg:npm/waived@1.0.0" {
		t.Errorf("excepted = %v", res.Excepted)
	}
	if res.Checked != 8 { // first-party and file entries are not counted
		t.Errorf("checked = %d, want 8", res.Checked)
	}
}

func sbomOf(comps string) string {
	return `{"bomFormat":"CycloneDX","components":[` + comps + `]}`
}

func TestExceptionIsExact(t *testing.T) {
	cases := []struct {
		name       string
		comp       string
		wantFind   int
		wantExcept int
		wantUnused int
	}{
		{"exact purl and licence",
			`{"type":"library","name":"waived","version":"1.0.0","purl":"pkg:npm/waived@1.0.0","licenses":[{"license":{"id":"Zlib"}}]}`, 0, 1, 0},
		{"same purl different licence",
			`{"type":"library","name":"waived","version":"1.0.0","purl":"pkg:npm/waived@1.0.0","licenses":[{"license":{"id":"AGPL-3.0-only"}}]}`, 1, 0, 1},
		{"same name and version other ecosystem",
			`{"type":"library","name":"waived","version":"1.0.0","purl":"pkg:pypi/waived@1.0.0","licenses":[{"license":{"id":"Zlib"}}]}`, 1, 0, 1},
		{"no purl",
			`{"type":"library","name":"waived","version":"1.0.0","licenses":[{"license":{"id":"Zlib"}}]}`, 1, 0, 1},
		{"other version",
			`{"type":"library","name":"waived","version":"1.0.1","purl":"pkg:npm/waived@1.0.1","licenses":[{"license":{"id":"Zlib"}}]}`, 1, 0, 1},
		{"licence compared after whitespace normalization",
			`{"type":"library","name":"waived","version":"1.0.0","purl":"pkg:npm/waived@1.0.0","licenses":[{"license":{"name":"Zlib"}}]}`, 0, 1, 0},
		{"unused exception fails",
			`{"type":"library","name":"ok","version":"1","purl":"pkg:npm/ok@1","licenses":[{"license":{"id":"MIT"}}]}`, 0, 0, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := mustPolicy(t).Check([]string{writeSBOM(t, sbomOf(c.comp))})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Findings) != c.wantFind || len(res.Excepted) != c.wantExcept || len(res.Unusedexc) != c.wantUnused {
				t.Errorf("findings=%v excepted=%v unused=%v", res.Findings, res.Excepted, res.Unusedexc)
			}
			if res.OK() != (c.wantFind == 0 && c.wantUnused == 0) {
				t.Errorf("OK() = %v", res.OK())
			}
		})
	}
}

func TestCheckRefusesEmptyOrWrongFormat(t *testing.T) {
	p := mustPolicy(t)
	for name, body := range map[string]string{
		"empty array":           `{"bomFormat":"CycloneDX","components":[]}`,
		"no components":         `{"bomFormat":"CycloneDX"}`,
		"only files":            sbomOf(`{"type":"file","name":"/a"},{"type":"file","name":"/b"}`),
		"only first-party":      sbomOf(`{"type":"library","name":"self","version":"1","purl":"pkg:npm/self@1"},{"type":"library","name":"example.test/self","purl":"pkg:golang/example.test/self@v1.0.0"}`),
		"files and first-party": sbomOf(`{"type":"file","name":"/a"},{"type":"library","name":"self","purl":"pkg:npm/self@1"}`),
		"wrong format":          `{"bomFormat":"SPDX","components":[{"type":"library","name":"a"}]}`,
		"not json":              `not json`,
		"too deep":              strings.Repeat(`{"type":"library","name":"a","components":[`, 40) + strings.Repeat(`]}`, 40),
	} {
		if _, err := p.Check([]string{writeSBOM(t, body)}); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := p.Check([]string{"/nonexistent.cdx.json"}); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestOneEmptySBOMFailsTheRun(t *testing.T) {
	good := writeSBOM(t, sbomOf(`{"type":"library","name":"ok","version":"1","licenses":[{"license":{"id":"MIT"}}]}`))
	empty := writeSBOM(t, sbomOf(`{"type":"file","name":"/a"}`))
	if _, err := mustPolicy(t).Check([]string{good, empty}); err == nil {
		t.Error("an SBOM without evaluated components must fail even when others pass")
	}
}

func TestNestedComponents(t *testing.T) {
	body := `{"bomFormat":"CycloneDX",
	 "metadata":{"component":{"type":"application","name":"root","components":[
	    {"type":"library","name":"meta-child","version":"1","licenses":[{"license":{"id":"GPL-3.0-only"}}]}]}},
	 "components":[
	  {"type":"library","name":"top","version":"1","licenses":[{"license":{"id":"MIT"}}],"components":[
	    {"type":"library","name":"nested","version":"1","licenses":[{"license":{"id":"AGPL-3.0-only"}}],"components":[
	      {"type":"library","name":"deep","version":"1","licenses":[{"license":{"id":"MIT"}}]}]}]},
	  {"type":"library","name":"self","version":"1","purl":"pkg:npm/self@1","components":[
	    {"type":"library","name":"under-first-party","version":"1","licenses":[{"license":{"id":"SSPL-1.0"}}]}]},
	  {"type":"file","name":"/f","components":[
	    {"type":"library","name":"under-file","version":"1"}]}
	 ]}`
	res, err := mustPolicy(t).Check([]string{writeSBOM(t, body)})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, f := range res.Findings {
		got[f.Package] = true
	}
	for _, n := range []string{"meta-child", "nested", "under-first-party", "under-file"} {
		if !got[n] {
			t.Errorf("%s not reported: %v", n, res.Findings)
		}
	}
	if len(res.Findings) != 4 {
		t.Errorf("findings = %v", res.Findings)
	}
	// top, nested, deep, meta-child, under-first-party, under-file; the root is not counted.
	if res.Checked != 6 {
		t.Errorf("checked = %d, want 6 (root, file and first-party entries are not counted)", res.Checked)
	}
}

func TestFirstPartyMatchesPurlNotName(t *testing.T) {
	cases := []struct {
		name string
		comp string
		skip bool
	}{
		{"exact npm purl", `{"type":"library","name":"self","version":"1","purl":"pkg:npm/self@1"}`, true},
		{"versionless purl", `{"type":"library","name":"example.test/self","purl":"pkg:golang/example.test/self"}`, true},
		{"same name other ecosystem", `{"type":"library","name":"self","version":"1","purl":"pkg:pypi/self@1"}`, false},
		{"same name no purl", `{"type":"library","name":"self","version":"1"}`, false},
		{"prefix is not a match", `{"type":"library","name":"self-evil","version":"1","purl":"pkg:npm/self-evil@1"}`, false},
		{"longer path is not a match", `{"type":"library","name":"example.test/self/v2","version":"v2.0.0","purl":"pkg:golang/example.test/self/v2@v2.0.0"}`, false},
	}
	ok := `{"type":"library","name":"ok","version":"1","licenses":[{"license":{"id":"MIT"}}]}`
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := mustPolicy(t).Check([]string{writeSBOM(t, sbomOf(ok+","+c.comp))})
			if err != nil {
				t.Fatal(err)
			}
			wantChecked := 2
			if c.skip {
				wantChecked = 1
			}
			if res.Checked != wantChecked {
				t.Errorf("checked = %d, want %d", res.Checked, wantChecked)
			}
		})
	}
}

func TestUnusedExceptionReported(t *testing.T) {
	p := mustPolicy(t)
	body := sbomOf(`{"type":"library","name":"ok","version":"1","licenses":[{"license":{"id":"MIT"}}]}`)
	res, err := p.Check([]string{writeSBOM(t, body)})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Unusedexc) != 1 || len(res.Findings) != 0 || res.OK() {
		t.Errorf("unused=%v findings=%v ok=%v", res.Unusedexc, res.Findings, res.OK())
	}
}

func TestRealPolicyFileLoads(t *testing.T) {
	p, err := LoadPolicy("../../supply-chain/license-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	for expr, want := range map[string]Verdict{"MIT": Allow, "MPL-2.0": Allow, "GPL-3.0-only": Denied, "LGPL-2.1-only": Denied, "BUSL-1.1": Denied, "NOASSERTION": Denied, "Zlib": Unlisted} {
		if got, _ := p.EvalExpression(expr); got != want {
			t.Errorf("%s = %v, want %v", expr, got, want)
		}
	}
}

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
  "first_party": ["example.test/self"],
  "exceptions": [{"package":"waived","version":"1.0.0","reason":"fixture","reviewer":"tester"}]
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
		"version":       `{"version":2}`,
		"unknown":       `{"version":1,"extra":1}`,
		"both":          `{"version":1,"allowed":[{"id":"GPL-3.0-only"}],"denied_prefixes":["GPL-"]}`,
		"exc no why":    `{"version":1,"exceptions":[{"package":"a","version":"1","reviewer":"x"}]}`,
		"exc no review": `{"version":1,"exceptions":[{"package":"a","version":"1","reason":"x"}]}`,
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
 {"type":"library","name":"waived","version":"1.0.0"},
 {"type":"library","name":"waived","version":"1.0.1"},
 {"type":"library","name":"example.test/self","version":"UNKNOWN"},
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
		"waived@1.0.1": "no licence declared (unknown)",
	}
	if len(got) != len(want) {
		t.Fatalf("findings = %v, want %v", got, want)
	}
	for k, r := range want {
		if got[k] != r {
			t.Errorf("%s: %q, want %q", k, got[k], r)
		}
	}
	if len(res.Excepted) != 1 || res.Excepted[0] != "waived@1.0.0" {
		t.Errorf("excepted = %v", res.Excepted)
	}
	if res.Checked != 8 { // first-party and file entries are not counted
		t.Errorf("checked = %d, want 8", res.Checked)
	}
}

func TestCheckRefusesEmptyOrWrongFormat(t *testing.T) {
	p := mustPolicy(t)
	for _, body := range []string{
		`{"bomFormat":"CycloneDX","components":[]}`,
		`{"bomFormat":"SPDX","components":[{"type":"library","name":"a"}]}`,
		`not json`,
	} {
		if _, err := p.Check([]string{writeSBOM(t, body)}); err == nil {
			t.Errorf("expected error for %q", body)
		}
	}
	if _, err := p.Check([]string{"/nonexistent.cdx.json"}); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestUnusedExceptionReported(t *testing.T) {
	p := mustPolicy(t)
	body := `{"bomFormat":"CycloneDX","components":[{"type":"library","name":"ok","version":"1","licenses":[{"license":{"id":"MIT"}}]}]}`
	res, err := p.Check([]string{writeSBOM(t, body)})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Unusedexc) != 1 || len(res.Findings) != 0 {
		t.Errorf("unused=%v findings=%v", res.Unusedexc, res.Findings)
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

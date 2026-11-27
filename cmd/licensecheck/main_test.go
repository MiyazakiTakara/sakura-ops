package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const policy = `{"version":1,"allowed":[{"id":"MIT"}],"denied_ids":[],"denied_prefixes":["GPL-"],
 "first_party":["pkg:npm/self"],
 "exceptions":[%s]}`

const excWaived = `{"purl":"pkg:npm/waived@1.0.0","license":"Zlib","reason":"fixture","reviewer":"tester"}`

func write(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestExitCodes(t *testing.T) {
	good := `{"bomFormat":"CycloneDX","components":[{"type":"library","name":"a","version":"1","purl":"pkg:npm/a@1","licenses":[{"license":{"id":"MIT"}}]}]}`
	bad := `{"bomFormat":"CycloneDX","components":[{"type":"library","name":"a","version":"1","purl":"pkg:npm/a@1","licenses":[{"license":{"id":"GPL-3.0-only"}}]}]}`
	onlyFirstParty := `{"bomFormat":"CycloneDX","components":[{"type":"library","name":"self","version":"1","purl":"pkg:npm/self@1"},{"type":"file","name":"/x"}]}`
	waived := `{"bomFormat":"CycloneDX","components":[{"type":"library","name":"waived","version":"1.0.0","purl":"pkg:npm/waived@1.0.0","licenses":[{"license":{"id":"Zlib"}}]}]}`
	cases := []struct {
		name, pol, sbom string
		want            int
	}{
		{"pass", "", good, 0},
		{"finding", "", bad, 1},
		{"unused exception", excWaived, good, 1},
		{"used exception passes", excWaived, waived, 0},
		{"zero evaluated", "", onlyFirstParty, 2},
		{"bad policy", `{"purl":"x"}`, good, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pol := write(t, "p.json", strings.Replace(policy, "%s", c.pol, 1))
			if c.name == "bad policy" {
				pol = write(t, "p.json", `{"version":1,"exceptions":[`+c.pol+`]}`)
			}
			sb := write(t, "s.cdx.json", c.sbom)
			var out, errb bytes.Buffer
			if got := run([]string{"-policy", pol, sb}, &out, &errb); got != c.want {
				t.Errorf("exit %d, want %d\nstdout: %s\nstderr: %s", got, c.want, out.String(), errb.String())
			}
		})
	}
	var out, errb bytes.Buffer
	if got := run(nil, &out, &errb); got != 2 {
		t.Errorf("no args: exit %d, want 2", got)
	}
}

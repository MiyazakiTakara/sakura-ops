package imagescan

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 6, 1, 15, 0, 0, 0, time.UTC)

const (
	dg    = "sha256:64c8b0d31e4ab6ea665e8426ea406a94bf52de3c05b3a4a8bf713704eefa9328"
	other = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	entry = `{"id":"CVE-2099-0001","image":"hub","digest":"` + dg + `","package":"libfoo","version":"1","reason":"r","reviewer":"x","expires":"%s"}`
)

func ign(t *testing.T, exp string) *IgnoreFile {
	t.Helper()
	f, err := ParseIgnore([]byte(`{"ignores":[` + strings.Replace(entry, "%s", exp, 1) + `]}`))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func finding(fixed ...string) Finding {
	return Finding{Image: "hub", Digest: dg, ID: "CVE-2099-0001", Severity: "HIGH", Package: "libfoo", Version: "1", Fixed: fixed}
}

func TestParseIgnoreRejects(t *testing.T) {
	full := func(k, v string) string {
		m := map[string]string{"id": "a", "image": "hub", "digest": dg, "package": "p", "version": "1",
			"reason": "r", "reviewer": "x", "expires": "2030-01-01"}
		if v == "" {
			delete(m, k)
		} else {
			m[k] = v
		}
		var parts []string
		for key, val := range m {
			parts = append(parts, `"`+key+`":"`+val+`"`)
		}
		return `{"ignores":[{` + strings.Join(parts, ",") + `}]}`
	}
	cases := map[string]string{
		"missing reviewer": full("reviewer", ""),
		"missing expiry":   full("expires", ""),
		"missing digest":   full("digest", ""),
		"missing version":  full("version", ""),
		"bad date":         full("expires", "soon"),
		"wildcard package": full("package", "*"),
		"wildcard version": full("version", "1.*"),
		"unknown field":    `{"ignores":[{"id":"a","image":"hub","digest":"d","package":"p","version":"1","reason":"r","reviewer":"x","expires":"2030-01-01","severity":"low"}]}`,
		"duplicate": `{"ignores":[` + strings.TrimSuffix(strings.TrimPrefix(full("id", "a"), `{"ignores":[`), `]}`) + `,` +
			strings.TrimSuffix(strings.TrimPrefix(full("expires", "2031-01-01"), `{"ignores":[`), `]}`) + `]}`,
		"trailing": `{"ignores":[]} {}`,
		"not json": `nope`,
	}
	for name, in := range cases {
		if _, err := ParseIgnore([]byte(in)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := ParseIgnore([]byte(full("id", "a"))); err != nil {
		t.Errorf("complete entry rejected: %v", err)
	}
}

func TestEvaluate(t *testing.T) {
	t.Run("fixable fails", func(t *testing.T) {
		r := Evaluate([]Finding{finding("2")}, &IgnoreFile{}, now)
		if r.OK() || len(r.Failures) != 1 {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("unfixed warns", func(t *testing.T) {
		r := Evaluate([]Finding{finding()}, &IgnoreFile{}, now)
		if !r.OK() || len(r.Warnings) != 1 {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("ignored passes", func(t *testing.T) {
		r := Evaluate([]Finding{finding("2")}, ign(t, "2026-06-30"), now)
		if !r.OK() || len(r.Suppressed) != 1 {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("valid on expiry day", func(t *testing.T) {
		if r := Evaluate([]Finding{finding("2")}, ign(t, "2026-06-01"), now); !r.OK() {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("expired fails and does not suppress", func(t *testing.T) {
		r := Evaluate([]Finding{finding("2")}, ign(t, "2026-05-31"), now)
		if r.OK() || len(r.Failures) != 1 || len(r.Problems) != 1 {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("unused fails", func(t *testing.T) {
		r := Evaluate(nil, ign(t, "2026-06-30"), now)
		if r.OK() || len(r.Problems) != 1 {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("other image or package does not match", func(t *testing.T) {
		f := finding("2")
		f.Image = "postgres"
		r := Evaluate([]Finding{f}, ign(t, "2026-06-30"), now)
		if r.OK() || len(r.Failures) != 1 {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("same CVE different version is not suppressed", func(t *testing.T) {
		f := finding("2")
		f.Version = "2"
		r := Evaluate([]Finding{f}, ign(t, "2026-06-30"), now)
		if r.OK() || len(r.Failures) != 1 || len(r.Suppressed) != 0 {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("different digest is not suppressed", func(t *testing.T) {
		f := finding("2")
		f.Digest = other
		r := Evaluate([]Finding{f}, ign(t, "2026-06-30"), now)
		if r.OK() || len(r.Failures) != 1 || len(r.Suppressed) != 0 {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("related id matches", func(t *testing.T) {
		f := finding("2")
		f.ID, f.Related = "GHSA-xxxx", []string{"CVE-2099-0001"}
		if r := Evaluate([]Finding{f}, ign(t, "2026-06-30"), now); !r.OK() {
			t.Fatalf("%+v", r)
		}
	})
}

func report(mut func(m map[string]any)) []byte {
	m := map[string]any{
		"matches": []any{
			map[string]any{"vulnerability": map[string]any{"id": "CVE-1", "severity": "High", "fix": map[string]any{"versions": []string{"2"}, "state": "fixed"}},
				"artifact": map[string]any{"name": "a", "version": "1"}},
			map[string]any{"vulnerability": map[string]any{"id": "CVE-2", "severity": "Critical", "fix": map[string]any{"versions": []string{}, "state": "not-fixed"}},
				"artifact": map[string]any{"name": "b", "version": "1"}},
			map[string]any{"vulnerability": map[string]any{"id": "CVE-3", "severity": "Medium", "fix": map[string]any{"versions": []string{"2"}, "state": "fixed"}},
				"artifact": map[string]any{"name": "c", "version": "1"}},
		},
		"source": map[string]any{"type": "image", "target": map[string]any{
			"imageID": "sha256:aaaa", "manifestDigest": other,
			"repoDigests": []string{"index.docker.io/library/postgres@" + dg}}},
		"descriptor": map[string]any{"name": "grype", "version": "0.122.1"},
	}
	if mut != nil {
		mut(m)
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return b
}

var (
	extWant   = Expect{GrypeVersion: "0.122.1", Digest: dg}
	localWant = Expect{GrypeVersion: "0.122.1", ImageID: "sha256:aaaa"}
)

func TestParseReportAccepts(t *testing.T) {
	r, err := ParseReport("pg", report(nil), extWant)
	if err != nil || len(r.Findings) != 2 || r.Matches != 3 {
		t.Fatalf("%v %+v", err, r)
	}
	if len(r.Findings[0].Fixed) == 0 || len(r.Findings[1].Fixed) != 0 || r.Findings[0].Digest != dg {
		t.Errorf("fix detection or identity wrong: %+v", r.Findings)
	}
	if _, err := ParseReport("hub", report(nil), localWant); err != nil {
		t.Errorf("local image ID: %v", err)
	}
	empty := report(func(m map[string]any) { m["matches"] = []any{} })
	if r, err := ParseReport("pg", empty, extWant); err != nil || r.Matches != 0 {
		t.Errorf("empty matches array must be a valid clean scan: %v", err)
	}
	viaManifest := report(func(m map[string]any) {
		m["source"].(map[string]any)["target"].(map[string]any)["repoDigests"] = []string{}
		m["source"].(map[string]any)["target"].(map[string]any)["manifestDigest"] = dg
	})
	if _, err := ParseReport("pg", viaManifest, extWant); err != nil {
		t.Errorf("manifest digest: %v", err)
	}
}

func TestParseReportRejects(t *testing.T) {
	set := func(k string, v any) []byte { return report(func(m map[string]any) { m[k] = v }) }
	del := func(k string) []byte { return report(func(m map[string]any) { delete(m, k) }) }
	badMatch := func(k, v string) []byte {
		return report(func(m map[string]any) {
			mm := m["matches"].([]any)[0].(map[string]any)
			if k == "id" || k == "severity" {
				mm["vulnerability"].(map[string]any)[k] = v
			} else {
				mm["artifact"].(map[string]any)[k] = v
			}
		})
	}
	src := func(f func(s map[string]any)) []byte {
		return report(func(m map[string]any) { f(m["source"].(map[string]any)) })
	}
	good := string(report(nil))
	cases := []struct {
		name string
		in   []byte
		want Expect
	}{
		{"garbage", []byte("garbage"), extWant},
		{"empty input", nil, extWant},
		{"truncated JSON", []byte(good[:len(good)/2]), extWant},
		{"trailing data", []byte(good + ` {}`), extWant},
		{"empty object", []byte(`{}`), extWant},
		{"empty source only", []byte(`{"source":{}}`), extWant},
		{"matches only", []byte(`{"matches":[]}`), extWant},
		{"matches null", set("matches", nil), extWant},
		{"matches missing", del("matches"), extWant},
		{"source missing", del("source"), extWant},
		{"source empty", set("source", map[string]any{}), extWant},
		{"source type dir", src(func(s map[string]any) { s["type"] = "directory" }), extWant},
		{"source target missing", src(func(s map[string]any) { delete(s, "target") }), extWant},
		{"descriptor missing", del("descriptor"), extWant},
		{"descriptor name", set("descriptor", map[string]any{"name": "trivy", "version": "0.122.1"}), extWant},
		{"descriptor version", set("descriptor", map[string]any{"name": "grype", "version": "0.1.0"}), extWant},
		{"wrong digest", report(nil), Expect{GrypeVersion: "0.122.1", Digest: other[:len(other)-1] + "1"}},
		{"digest only as prefix", src(func(s map[string]any) {
			s["target"].(map[string]any)["repoDigests"] = []string{"postgres@" + dg + "0"}
			s["target"].(map[string]any)["manifestDigest"] = other
		}), extWant},
		{"wrong image ID", report(nil), Expect{GrypeVersion: "0.122.1", ImageID: "sha256:bbbb"}},
		{"local ID only as repo digest prefix", src(func(s map[string]any) {
			s["target"].(map[string]any)["imageID"] = "sha256:cfg"
			s["target"].(map[string]any)["repoDigests"] = []string{"sakuraops-image-scan-hub@sha256:aaaa0"}
		}), localWant},
		{"local ID only as manifest digest", src(func(s map[string]any) {
			s["target"].(map[string]any)["imageID"] = "sha256:cfg"
			s["target"].(map[string]any)["manifestDigest"] = "sha256:aaaa"
			s["target"].(map[string]any)["repoDigests"] = []string{}
		}), localWant},
		{"external report offered as local ID without imageID", src(func(s map[string]any) { delete(s["target"].(map[string]any), "imageID") }), localWant},
		{"no expectation", report(nil), Expect{GrypeVersion: "0.122.1"}},
		{"both expectations", report(nil), Expect{GrypeVersion: "0.122.1", Digest: dg, ImageID: "sha256:aaaa"}},
		{"no grype version expectation", report(nil), Expect{Digest: dg}},
		{"match without id", badMatch("id", ""), extWant},
		{"match without severity", badMatch("severity", ""), extWant},
		{"match without artifact name", badMatch("name", ""), extWant},
		{"match without artifact version", badMatch("version", ""), extWant},
	}
	for _, c := range cases {
		if _, err := ParseReport("pg", c.in, c.want); err == nil {
			t.Errorf("%s: expected error", c.name)
		}
	}
}

func TestSummarize(t *testing.T) {
	a := &Report{Image: "a", Matches: 5}
	b := &Report{Image: "b", Matches: 2}
	res := Result{
		Failures: []Finding{{Image: "a", ID: "CVE-1"}},
		Warnings: []Finding{{Image: "a", ID: "CVE-1"}, {Image: "b", ID: "CVE-2"}},
	}
	per, distinct := Summarize([]*Report{a, b}, res)
	if distinct != 2 || per[0].Fixable != 1 || per[0].Warnings != 1 || per[1].Warnings != 1 || per[0].Matches != 5 {
		t.Fatalf("%+v %d", per, distinct)
	}
}

func TestTrimmedRealFixture(t *testing.T) {
	b, err := os.ReadFile("testdata/grype-postgres-trimmed.json")
	if err != nil {
		t.Fatal(err)
	}
	r, err := ParseReport("postgres", b, extWant)
	if err != nil || r.Matches != 2 || len(r.Findings) != 1 {
		t.Fatalf("%v %+v", err, r)
	}
}

// Containerd image store (CI runners): `docker image inspect .Id` is the index
// digest, which grype reports in repoDigests, not as imageID.
func TestParseReportAcceptsContainerdLocalID(t *testing.T) {
	id := "sha256:" + strings.Repeat("c", 64)
	in := report(func(m map[string]any) {
		tg := m["source"].(map[string]any)["target"].(map[string]any)
		tg["imageID"] = "sha256:" + strings.Repeat("e", 64)
		tg["repoDigests"] = []string{"sakuraops-image-scan-hub@" + id}
	})
	r, err := ParseReport("hub", in, Expect{GrypeVersion: "0.122.1", ImageID: id})
	if err != nil || r.Identity != id {
		t.Fatalf("%v %+v", err, r)
	}
}

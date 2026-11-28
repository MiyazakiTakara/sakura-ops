package imagescan

import (
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 6, 1, 15, 0, 0, 0, time.UTC)

const entry = `{"id":"CVE-2099-0001","image":"hub","package":"libfoo","reason":"r","reviewer":"x","expires":"%s"}`

func ign(t *testing.T, exp string) *IgnoreFile {
	t.Helper()
	f, err := ParseIgnore([]byte(`{"ignores":[` + strings.Replace(entry, "%s", exp, 1) + `]}`))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func finding(fixed ...string) Finding {
	return Finding{Image: "hub", ID: "CVE-2099-0001", Severity: "HIGH", Package: "libfoo", Version: "1", Fixed: fixed}
}

func TestParseIgnoreRejects(t *testing.T) {
	cases := map[string]string{
		"missing reviewer": `{"ignores":[{"id":"a","image":"hub","package":"p","reason":"r","expires":"2030-01-01"}]}`,
		"missing expiry":   `{"ignores":[{"id":"a","image":"hub","package":"p","reason":"r","reviewer":"x"}]}`,
		"bad date":         `{"ignores":[{"id":"a","image":"hub","package":"p","reason":"r","reviewer":"x","expires":"soon"}]}`,
		"wildcard package": `{"ignores":[{"id":"a","image":"hub","package":"*","reason":"r","reviewer":"x","expires":"2030-01-01"}]}`,
		"unknown field":    `{"ignores":[{"id":"a","image":"hub","package":"p","reason":"r","reviewer":"x","expires":"2030-01-01","severity":"low"}]}`,
		"duplicate":        `{"ignores":[` + strings.Repeat(`{"id":"a","image":"hub","package":"p","reason":"r","reviewer":"x","expires":"2030-01-01"},`, 1) + `{"id":"a","image":"hub","package":"p","reason":"r","reviewer":"x","expires":"2031-01-01"}]}`,
		"trailing":         `{"ignores":[]} {}`,
		"not json":         `nope`,
	}
	for name, in := range cases {
		if _, err := ParseIgnore([]byte(in)); err == nil {
			t.Errorf("%s: expected error", name)
		}
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
	t.Run("related id matches", func(t *testing.T) {
		f := finding("2")
		f.ID, f.Related = "GHSA-xxxx", []string{"CVE-2099-0001"}
		if r := Evaluate([]Finding{f}, ign(t, "2026-06-30"), now); !r.OK() {
			t.Fatalf("%+v", r)
		}
	})
}

func TestParseReport(t *testing.T) {
	if _, err := ParseReport("hub", []byte(`{"matches":[]}`)); err == nil {
		t.Error("report without source must be rejected")
	}
	if _, err := ParseReport("hub", []byte(`garbage`)); err == nil {
		t.Error("garbage must be rejected")
	}
	rep := `{"source":{},"matches":[
	{"vulnerability":{"id":"CVE-1","severity":"High","fix":{"versions":["2"],"state":"fixed"}},"artifact":{"name":"a","version":"1"}},
	{"vulnerability":{"id":"CVE-2","severity":"Critical","fix":{"versions":[],"state":"not-fixed"}},"artifact":{"name":"b","version":"1"}},
	{"vulnerability":{"id":"CVE-3","severity":"Medium","fix":{"versions":["2"],"state":"fixed"}},"artifact":{"name":"c","version":"1"}}]}`
	f, err := ParseReport("hub", []byte(rep))
	if err != nil || len(f) != 2 {
		t.Fatalf("%v %+v", err, f)
	}
	if len(f[0].Fixed) == 0 || len(f[1].Fixed) != 0 {
		t.Errorf("fix detection wrong: %+v", f)
	}
}

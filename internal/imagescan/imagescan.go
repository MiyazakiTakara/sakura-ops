// Package imagescan evaluates grype JSON reports against the reviewed ignore
// file. Policy: HIGH or CRITICAL findings with a fix available fail; unfixed
// HIGH or CRITICAL findings are warnings. Ignore entries need an id, image,
// package, reason, reviewer and an unexpired date; unused and expired entries
// fail. Findings of the direct and the SBOM-mode scan of an image are united.
// Nothing here lowers severities or short-circuits a failure.
package imagescan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

// IgnoreEntry is one reviewed exception for exactly one finding: advisory id,
// image label, image identity (repo digest for external images; the image ID
// for locally built ones, which changes with every build, so local exceptions
// are per build and discouraged), package name and exact package version.
type IgnoreEntry struct {
	ID       string `json:"id"`
	Image    string `json:"image"`
	Digest   string `json:"digest"`
	Package  string `json:"package"`
	Version  string `json:"version"`
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
		for name, v := range map[string]string{"id": e.ID, "image": e.Image, "digest": e.Digest, "package": e.Package, "version": e.Version,
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
		key := e.ID + "|" + e.Image + "|" + e.Digest + "|" + e.Package + "|" + e.Version
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
	Digest   string // identity of the scanned image (repo digest or local image ID)
	ID       string
	Related  []string
	Severity string
	Package  string
	Version  string
	Fixed    []string
}

// Expect is what a report must prove about itself.
type Expect struct {
	GrypeVersion string // pinned grype version, e.g. 0.122.1
	Digest       string // external image: pinned sha256:<hex>; empty for local images
	ImageID      string // locally built image: docker image ID sha256:<hex>; empty for external
}

// Report is the validated, summarised content of one grype report.
type Report struct {
	Image    string
	Identity string
	Matches  int // all matches, any severity
	Findings []Finding
}

type grypeMatch struct {
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
}

type grypeReport struct {
	Matches *[]grypeMatch `json:"matches"`
	Source  *struct {
		Type   string `json:"type"`
		Target *struct {
			ImageID        string   `json:"imageID"`
			ManifestDigest string   `json:"manifestDigest"`
			RepoDigests    []string `json:"repoDigests"`
		} `json:"target"`
	} `json:"source"`
	Descriptor *struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"descriptor"`
}

// ParseReport validates a grype JSON report and extracts its HIGH and CRITICAL
// findings. It is deliberately strict so that an empty, truncated, foreign or
// wrong-image file cannot pass as a clean scan: the document must be a single
// JSON value with a non-null matches array, an image source whose identity
// equals the expected one, a grype descriptor of the pinned version, and every
// match must carry id, severity, package name and version.
func ParseReport(image string, b []byte, want Expect) (*Report, error) {
	fail := func(format string, a ...any) (*Report, error) {
		return nil, fmt.Errorf("%s: %s", image, fmt.Sprintf(format, a...))
	}
	if (want.Digest == "") == (want.ImageID == "") {
		return fail("exactly one of expected digest or expected image ID is required")
	}
	if want.GrypeVersion == "" {
		return fail("expected grype version is required")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	var r grypeReport
	if err := dec.Decode(&r); err != nil {
		return fail("parse grype report: %v", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return fail("parse grype report: trailing data")
	}
	if r.Descriptor == nil || r.Descriptor.Name != "grype" {
		return fail("not a grype report (descriptor.name)")
	}
	if r.Descriptor.Version != want.GrypeVersion {
		return fail("grype version %q, want %q", r.Descriptor.Version, want.GrypeVersion)
	}
	if r.Source == nil || r.Source.Type != "image" || r.Source.Target == nil {
		return fail("report source is missing or not an image")
	}
	t := r.Source.Target
	identity, err := matchIdentity(t.ImageID, t.ManifestDigest, t.RepoDigests, want)
	if err != nil {
		return fail("%v", err)
	}
	if r.Matches == nil {
		return fail("report has no matches array (null or missing)")
	}
	rep := &Report{Image: image, Identity: identity, Matches: len(*r.Matches)}
	for i, m := range *r.Matches {
		if m.Vulnerability.ID == "" || m.Vulnerability.Severity == "" || m.Artifact.Name == "" || m.Artifact.Version == "" {
			return fail("match %d lacks vulnerability id, severity, artifact name or version", i)
		}
		sev := strings.ToUpper(m.Vulnerability.Severity)
		if sev != "HIGH" && sev != "CRITICAL" {
			continue
		}
		f := Finding{Image: image, Digest: identity, ID: m.Vulnerability.ID, Severity: sev,
			Package: m.Artifact.Name, Version: m.Artifact.Version}
		for _, rel := range m.Related {
			f.Related = append(f.Related, rel.ID)
		}
		f.Fixed = m.Vulnerability.Fix.Versions
		if len(f.Fixed) == 0 && strings.EqualFold(m.Vulnerability.Fix.State, "fixed") {
			f.Fixed = []string{"(fixed, version not listed)"}
		}
		rep.Findings = append(rep.Findings, f)
	}
	return rep, nil
}

// matchIdentity proves that a source (grype report or syft SBOM) describes the
// expected image and returns the identity that was matched.
func matchIdentity(imageID, manifestDigest string, repoDigests []string, want Expect) (string, error) {
	if want.ImageID != "" {
		// The classic Docker image store reports the config digest as the image
		// ID (grype: imageID). With the containerd image store (default on
		// current Docker, e.g. GitHub runners) `docker image inspect .Id` is the
		// image index digest, which grype lists in repoDigests as
		// "<name>@<digest>". Both are exact SHA-256 identities of the same image.
		ok := imageID == want.ImageID
		for _, d := range repoDigests {
			if strings.HasSuffix(d, "@"+want.ImageID) {
				ok = true
			}
		}
		if !ok {
			return "", fmt.Errorf("source image ID %q (repo digests %v) does not match expected %q", imageID, repoDigests, want.ImageID)
		}
		return want.ImageID, nil
	}
	ok := manifestDigest == want.Digest
	for _, d := range repoDigests {
		if strings.HasSuffix(d, "@"+want.Digest) {
			ok = true
		}
	}
	if !ok {
		return "", fmt.Errorf("source does not carry expected digest %s", want.Digest)
	}
	return want.Digest, nil
}

type syftSBOM struct {
	Artifacts *[]json.RawMessage `json:"artifacts"`
	Source    *struct {
		Type     string `json:"type"`
		Metadata *struct {
			ImageID        string   `json:"imageID"`
			ManifestDigest string   `json:"manifestDigest"`
			RepoDigests    []string `json:"repoDigests"`
		} `json:"metadata"`
	} `json:"source"`
	Descriptor *struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"descriptor"`
}

// VerifySBOM checks that a syft-json SBOM was produced by the pinned syft from
// the expected image (same identity rules as ParseReport) and is not empty.
// The SBOM is the input of the sbom-mode grype scan, so its own source identity
// is verified independently of what grype copies into its report.
func VerifySBOM(image string, b []byte, want Expect, syftVersion string) error {
	fail := func(format string, a ...any) error {
		return fmt.Errorf("%s: syft SBOM: %s", image, fmt.Sprintf(format, a...))
	}
	if (want.Digest == "") == (want.ImageID == "") {
		return fail("exactly one of expected digest or expected image ID is required")
	}
	if syftVersion == "" {
		return fail("expected syft version is required")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	var s syftSBOM
	if err := dec.Decode(&s); err != nil {
		return fail("parse: %v", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return fail("trailing data")
	}
	if s.Descriptor == nil || s.Descriptor.Name != "syft" {
		return fail("not a syft report (descriptor.name)")
	}
	if s.Descriptor.Version != syftVersion {
		return fail("syft version %q, want %q", s.Descriptor.Version, syftVersion)
	}
	if s.Source == nil || s.Source.Type != "image" || s.Source.Metadata == nil {
		return fail("source is missing or not an image")
	}
	m := s.Source.Metadata
	if _, err := matchIdentity(m.ImageID, m.ManifestDigest, m.RepoDigests, want); err != nil {
		return fail("%v", err)
	}
	if s.Artifacts == nil || len(*s.Artifacts) == 0 {
		return fail("no artifacts (null, missing or empty); refusing to scan an empty SBOM")
	}
	return nil
}

// ImageScan is both evaluated modes of one image: the direct image scan and
// the scan of the syft SBOM of the same image.
type ImageScan struct {
	Image    string
	Direct   *Report
	SBOM     *Report
	Findings []Finding // union, deduplicated by (image, id, package, version)
}

// Combine builds the union of the direct and the SBOM-mode report. Both are
// mandatory: a missing mode is an error, never a silent single-mode pass.
// Findings present in both modes are counted once; if the modes disagree the
// stricter view wins (higher severity, union of fix versions and related ids).
func Combine(image string, direct, sbom *Report) (*ImageScan, error) {
	if direct == nil {
		return nil, fmt.Errorf("%s: direct image report is missing", image)
	}
	if sbom == nil {
		return nil, fmt.Errorf("%s: SBOM-mode report is missing", image)
	}
	if direct.Image != image || sbom.Image != image || direct.Identity != sbom.Identity {
		return nil, fmt.Errorf("%s: direct (%s %s) and SBOM-mode (%s %s) reports do not describe the same image",
			image, direct.Image, direct.Identity, sbom.Image, sbom.Identity)
	}
	type key struct{ id, pkg, ver string }
	idx := map[key]int{}
	sc := &ImageScan{Image: image, Direct: direct, SBOM: sbom}
	for _, f := range append(append([]Finding{}, direct.Findings...), sbom.Findings...) {
		k := key{f.ID, f.Package, f.Version}
		i, dup := idx[k]
		if !dup {
			idx[k] = len(sc.Findings)
			sc.Findings = append(sc.Findings, f)
			continue
		}
		e := &sc.Findings[i]
		if f.Severity == "CRITICAL" {
			e.Severity = "CRITICAL"
		}
		e.Fixed = appendMissing(e.Fixed, f.Fixed)
		e.Related = appendMissing(e.Related, f.Related)
	}
	return sc, nil
}

func appendMissing(dst, src []string) []string {
	for _, s := range src {
		found := false
		for _, d := range dst {
			found = found || d == s
		}
		if !found {
			dst = append(dst, s)
		}
	}
	return dst
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
	if e.Image != f.Image || e.Digest != f.Digest || e.Package != f.Package || e.Version != f.Version {
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
			res.Problems = append(res.Problems, fmt.Sprintf("expired ignore entry: %s %s %s %s@%s (expired %s, reviewer %s)",
				e.ID, e.Image, e.Digest, e.Package, e.Version, e.Expires, e.Reviewer))
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
			res.Problems = append(res.Problems, fmt.Sprintf("unused ignore entry: %s %s %s %s@%s (matches no HIGH/CRITICAL finding; remove it)",
				e.ID, e.Image, e.Digest, e.Package, e.Version))
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

// ImageSummary counts findings of one image after policy evaluation.
type ImageSummary struct {
	Image         string
	DirectMatches int // all matches of the direct image scan, any severity
	SBOMMatches   int // all matches of the SBOM-mode scan, any severity
	DirectHigh    int // HIGH/CRITICAL findings per mode, before deduplication
	SBOMHigh      int
	UnionHigh     int // distinct (id, package, version) HIGH/CRITICAL findings
	Fixable       int
	Warnings      int
	Suppressed    int
}

// Summarize returns per-image counts (in scan order) and the number of
// distinct advisory ids among all HIGH/CRITICAL findings, ignored or not.
func Summarize(scans []*ImageScan, res Result) (perImage []ImageSummary, distinct int) {
	idx := map[string]int{}
	for _, sc := range scans {
		idx[sc.Image] = len(perImage)
		perImage = append(perImage, ImageSummary{Image: sc.Image, DirectMatches: sc.Direct.Matches, SBOMMatches: sc.SBOM.Matches,
			DirectHigh: len(sc.Direct.Findings), SBOMHigh: len(sc.SBOM.Findings), UnionHigh: len(sc.Findings)})
	}
	ids := map[string]bool{}
	count := func(fs []Finding, add func(*ImageSummary)) {
		for _, f := range fs {
			ids[f.ID] = true
			add(&perImage[idx[f.Image]])
		}
	}
	count(res.Failures, func(s *ImageSummary) { s.Fixable++ })
	count(res.Warnings, func(s *ImageSummary) { s.Warnings++ })
	count(res.Suppressed, func(s *ImageSummary) { s.Suppressed++ })
	return perImage, len(ids)
}

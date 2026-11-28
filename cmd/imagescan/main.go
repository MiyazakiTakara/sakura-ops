// Command imagescan derives the container image scan set from Compose and
// evaluates grype JSON reports against the reviewed ignore file.
//
//	imagescan plan -compose file
//	imagescan [-ignore file] -grype-version V label=report.json,digest=sha256:<hex>...
//	imagescan [-ignore file] -grype-version V label=report.json,id=sha256:<imageID>...
//
// "digest=" pins an external image, "id=" a locally built one. Exit codes: 0 pass,
// 1 findings or ignore-file problems, 2 usage, plan or report-validation error.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/MiyazakiTakara/sakura-ops/internal/imagescan"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, time.Now())) }

func run(args []string, stdout, stderr io.Writer, now time.Time) int {
	if len(args) > 0 && args[0] == "plan" {
		return runPlan(args[1:], stdout, stderr)
	}
	fs := flag.NewFlagSet("imagescan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	ignorePath := fs.String("ignore", "supply-chain/image-scan-ignore.json", "reviewed ignore file")
	grypeVersion := fs.String("grype-version", "", "pinned grype version every report must carry")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 || *grypeVersion == "" {
		fmt.Fprintln(stderr, "usage: imagescan [-ignore file] -grype-version V label=report.json,(digest|id)=sha256:<hex>...")
		fmt.Fprintln(stderr, "       imagescan plan -compose file")
		return 2
	}
	ign, err := imagescan.LoadIgnore(*ignorePath)
	if err != nil {
		fmt.Fprintln(stderr, "imagescan:", err)
		return 2
	}
	var reports []*imagescan.Report
	var all []imagescan.Finding
	seen := map[string]bool{}
	for _, a := range fs.Args() {
		label, path, want, err := parseArg(a, *grypeVersion)
		if err != nil {
			fmt.Fprintln(stderr, "imagescan:", err)
			return 2
		}
		if seen[label] {
			fmt.Fprintf(stderr, "imagescan: duplicate label %q\n", label)
			return 2
		}
		seen[label] = true
		b, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(stderr, "imagescan:", err)
			return 2
		}
		r, err := imagescan.ParseReport(label, b, want)
		if err != nil {
			fmt.Fprintln(stderr, "imagescan:", err)
			return 2
		}
		reports = append(reports, r)
		all = append(all, r.Findings...)
	}
	res := imagescan.Evaluate(all, ign, now)
	line := func(prefix string, f imagescan.Finding) {
		fmt.Fprintf(stdout, "%s %s %s %s %s@%s fixed in %v\n", prefix, f.Image, f.Severity, f.ID, f.Package, f.Version, f.Fixed)
	}
	for _, f := range res.Warnings {
		line("WARNING (unfixed):", f)
	}
	for _, f := range res.Suppressed {
		line("ignored (reviewed):", f)
	}
	for _, f := range res.Failures {
		line("FAIL:", f)
	}
	for _, p := range res.Problems {
		fmt.Fprintln(stdout, "FAIL:", p)
	}
	per, distinct := imagescan.Summarize(reports, res)
	for _, s := range per {
		fmt.Fprintf(stdout, "imagescan: %s: %d matches, %d fixable HIGH/CRITICAL, %d unfixed HIGH/CRITICAL warnings, %d ignored\n",
			s.Image, s.Matches, s.Fixable, s.Warnings, s.Suppressed)
	}
	fmt.Fprintf(stdout, "imagescan: total %d fixable, %d unfixed warnings, %d ignored, %d ignore-file problems; %d distinct HIGH/CRITICAL advisories\n",
		len(res.Failures), len(res.Warnings), len(res.Suppressed), len(res.Problems), distinct)
	if !res.OK() {
		return 1
	}
	return 0
}

func parseArg(a, grypeVersion string) (label, path string, want imagescan.Expect, err error) {
	want.GrypeVersion = grypeVersion
	label, rest, ok := strings.Cut(a, "=")
	if !ok || label == "" {
		return "", "", want, fmt.Errorf("argument %q is not label=report.json,(digest|id)=sha256:<hex>", a)
	}
	parts := strings.Split(rest, ",")
	path = parts[0]
	for _, p := range parts[1:] {
		k, v, _ := strings.Cut(p, "=")
		switch {
		case k == "digest" && want.Digest == "":
			want.Digest = v
		case k == "id" && want.ImageID == "":
			want.ImageID = v
		default:
			return "", "", want, fmt.Errorf("argument %q: unexpected %q", a, p)
		}
	}
	if path == "" || (want.Digest == "") == (want.ImageID == "") {
		return "", "", want, fmt.Errorf("argument %q needs a report path and exactly one of digest= or id=", a)
	}
	return label, path, want, nil
}

func runPlan(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("imagescan plan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	compose := fs.String("compose", "deployment/compose/compose.dev.yaml", "Compose file to derive the image set from")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		return 2
	}
	b, err := os.ReadFile(*compose)
	if err != nil {
		fmt.Fprintln(stderr, "imagescan:", err)
		return 2
	}
	p, err := imagescan.ParsePlan(b)
	if err != nil {
		fmt.Fprintln(stderr, "imagescan:", err)
		return 2
	}
	p.Write(stdout)
	return 0
}

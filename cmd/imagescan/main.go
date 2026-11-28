// Command imagescan derives the container image scan set from Compose and
// evaluates grype JSON reports against the reviewed ignore file.
//
//	imagescan plan -compose file
//	imagescan [-ignore file] -grype-version V -syft-version S label=direct.json,sbom-report=sbom.grype.json,syft=sbom.syft.json,digest=sha256:<hex>...
//	imagescan [-ignore file] -grype-version V -syft-version S label=direct.json,sbom-report=sbom.grype.json,syft=sbom.syft.json,id=sha256:<imageID>...
//
// Every image is evaluated in two modes whose findings are united and deduplicated:
// the direct grype report of the image (first path) and the grype report of the syft-json
// SBOM of the same image (sbom-report=, scanned from syft=). All three files are mandatory
// and validated against the same identity. "digest=" pins an external image, "id=" a
// locally built one. Exit codes: 0 pass, 1 findings or ignore-file problems, 2 usage,
// plan or report-validation error.
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
	syftVersion := fs.String("syft-version", "", "pinned syft version every SBOM must carry")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 || *grypeVersion == "" || *syftVersion == "" {
		fmt.Fprintln(stderr, "usage: imagescan [-ignore file] -grype-version V -syft-version S label=direct.json,sbom-report=r.json,syft=s.json,(digest|id)=sha256:<hex>...")
		fmt.Fprintln(stderr, "       imagescan plan -compose file")
		return 2
	}
	ign, err := imagescan.LoadIgnore(*ignorePath)
	if err != nil {
		fmt.Fprintln(stderr, "imagescan:", err)
		return 2
	}
	var scans []*imagescan.ImageScan
	var all []imagescan.Finding
	seen := map[string]bool{}
	for _, a := range fs.Args() {
		arg, err := parseArg(a, *grypeVersion)
		if err != nil {
			fmt.Fprintln(stderr, "imagescan:", err)
			return 2
		}
		if seen[arg.label] {
			fmt.Fprintf(stderr, "imagescan: duplicate label %q\n", arg.label)
			return 2
		}
		seen[arg.label] = true
		read := func(path string) []byte {
			b, rerr := os.ReadFile(path)
			if rerr != nil {
				fmt.Fprintln(stderr, "imagescan:", rerr)
				return nil
			}
			return b
		}
		directB, sbomB, syftB := read(arg.direct), read(arg.sbomReport), read(arg.syft)
		if directB == nil || sbomB == nil || syftB == nil {
			return 2
		}
		if err := imagescan.VerifySBOM(arg.label, syftB, arg.want, *syftVersion); err != nil {
			fmt.Fprintln(stderr, "imagescan:", err)
			return 2
		}
		direct, err := imagescan.ParseReport(arg.label, directB, arg.want)
		if err != nil {
			fmt.Fprintln(stderr, "imagescan: direct report:", err)
			return 2
		}
		sbom, err := imagescan.ParseReport(arg.label, sbomB, arg.want)
		if err != nil {
			fmt.Fprintln(stderr, "imagescan: SBOM-mode report:", err)
			return 2
		}
		sc, err := imagescan.Combine(arg.label, direct, sbom)
		if err != nil {
			fmt.Fprintln(stderr, "imagescan:", err)
			return 2
		}
		scans = append(scans, sc)
		all = append(all, sc.Findings...)
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
	per, distinct := imagescan.Summarize(scans, res)
	for _, s := range per {
		fmt.Fprintf(stdout, "imagescan: %s: direct %d matches (%d HIGH/CRITICAL), sbom %d matches (%d HIGH/CRITICAL), union %d distinct HIGH/CRITICAL: %d fixable, %d unfixed warnings, %d ignored\n",
			s.Image, s.DirectMatches, s.DirectHigh, s.SBOMMatches, s.SBOMHigh, s.UnionHigh, s.Fixable, s.Warnings, s.Suppressed)
	}
	fmt.Fprintf(stdout, "imagescan: total %d fixable, %d unfixed warnings, %d ignored, %d ignore-file problems; %d distinct HIGH/CRITICAL advisories\n",
		len(res.Failures), len(res.Warnings), len(res.Suppressed), len(res.Problems), distinct)
	if !res.OK() {
		return 1
	}
	return 0
}

type scanArg struct {
	label      string
	direct     string
	sbomReport string
	syft       string
	want       imagescan.Expect
}

func parseArg(a, grypeVersion string) (scanArg, error) {
	r := scanArg{}
	r.want.GrypeVersion = grypeVersion
	label, rest, ok := strings.Cut(a, "=")
	if !ok || label == "" {
		return r, fmt.Errorf("argument %q is not label=direct.json,sbom-report=...,syft=...,(digest|id)=sha256:<hex>", a)
	}
	r.label = label
	parts := strings.Split(rest, ",")
	r.direct = parts[0]
	for _, p := range parts[1:] {
		k, v, _ := strings.Cut(p, "=")
		switch {
		case k == "digest" && r.want.Digest == "":
			r.want.Digest = v
		case k == "id" && r.want.ImageID == "":
			r.want.ImageID = v
		case k == "sbom-report" && r.sbomReport == "":
			r.sbomReport = v
		case k == "syft" && r.syft == "":
			r.syft = v
		default:
			return r, fmt.Errorf("argument %q: unexpected %q", a, p)
		}
	}
	if r.direct == "" || r.sbomReport == "" || r.syft == "" || (r.want.Digest == "") == (r.want.ImageID == "") {
		return r, fmt.Errorf("argument %q needs a direct report path, sbom-report=, syft= (no single-mode scan) and exactly one of digest= or id=", a)
	}
	return r, nil
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

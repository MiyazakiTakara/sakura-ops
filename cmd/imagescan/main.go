// Command imagescan evaluates grype JSON reports against the reviewed ignore
// file. Usage: imagescan [-ignore file] label=report.json...
// Exit codes: 0 pass, 1 findings or ignore-file problems, 2 usage or input error.
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
	fs := flag.NewFlagSet("imagescan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	ignorePath := fs.String("ignore", "supply-chain/image-scan-ignore.json", "reviewed ignore file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "usage: imagescan [-ignore file] label=report.json...")
		return 2
	}
	ign, err := imagescan.LoadIgnore(*ignorePath)
	if err != nil {
		fmt.Fprintln(stderr, "imagescan:", err)
		return 2
	}
	var all []imagescan.Finding
	for _, a := range fs.Args() {
		label, path, ok := strings.Cut(a, "=")
		if !ok || label == "" || path == "" {
			fmt.Fprintf(stderr, "imagescan: argument %q is not label=report.json\n", a)
			return 2
		}
		b, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(stderr, "imagescan:", err)
			return 2
		}
		f, err := imagescan.ParseReport(label, b)
		if err != nil {
			fmt.Fprintln(stderr, "imagescan:", err)
			return 2
		}
		all = append(all, f...)
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
	fmt.Fprintf(stdout, "imagescan: %d fixable, %d unfixed warnings, %d ignored, %d ignore-file problems\n",
		len(res.Failures), len(res.Warnings), len(res.Suppressed), len(res.Problems))
	if !res.OK() {
		return 1
	}
	return 0
}

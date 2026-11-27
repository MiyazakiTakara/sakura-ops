// Command licensecheck fails when a CycloneDX SBOM contains a component whose
// licence is denied, unknown or not on the allowlist of the policy file, when
// a policy exception is stale, or when an SBOM yields no evaluated components.
//
// Exit codes: 0 pass, 1 policy failure (findings or unused exceptions),
// 2 usage, policy or SBOM error.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/MiyazakiTakara/sakura-ops/internal/licensecheck"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("licensecheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	policyPath := fs.String("policy", "supply-chain/license-policy.json", "policy file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "usage: licensecheck [-policy file] sbom.cdx.json...")
		return 2
	}
	p, err := licensecheck.LoadPolicy(*policyPath)
	if err != nil {
		fmt.Fprintln(stderr, "licensecheck:", err)
		return 2
	}
	res, err := p.Check(fs.Args())
	if err != nil {
		fmt.Fprintln(stderr, "licensecheck:", err)
		return 2
	}
	keys := make([]string, 0, len(res.Counts))
	for k := range res.Counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Fprintf(stdout, "checked %d components in %d SBOMs\n", res.Checked, fs.NArg())
	for _, k := range keys {
		fmt.Fprintf(stdout, "  %5d  %s\n", res.Counts[k], k)
	}
	for _, e := range res.Excepted {
		fmt.Fprintf(stdout, "excepted by policy: %s\n", e)
	}
	if res.OK() {
		fmt.Fprintln(stdout, "OK")
		return 0
	}
	if len(res.Unusedexc) > 0 {
		fmt.Fprintf(stdout, "\nFAIL: %d stale exception(s) match nothing; remove them from the policy\n", len(res.Unusedexc))
		for _, e := range res.Unusedexc {
			fmt.Fprintf(stdout, "  unused exception %s\n", e)
		}
	}
	if len(res.Findings) > 0 {
		fmt.Fprintf(stdout, "\nFAIL: %d component(s) violate the licence policy\n", len(res.Findings))
		for _, f := range res.Findings {
			fmt.Fprintf(stdout, "  %s\t%s\t%s\t%s\t(%s)\n", f.Package, f.Version, f.Licence, f.Reason, f.SBOM)
		}
	}
	return 1
}

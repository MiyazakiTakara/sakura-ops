// Command licensecheck fails when a CycloneDX SBOM contains a component whose
// licence is denied, unknown or not on the allowlist of the policy file.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/MiyazakiTakara/sakura-ops/internal/licensecheck"
)

func main() {
	policyPath := flag.String("policy", "supply-chain/license-policy.json", "policy file")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: licensecheck [-policy file] sbom.cdx.json...")
		os.Exit(2)
	}
	p, err := licensecheck.LoadPolicy(*policyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "licensecheck:", err)
		os.Exit(2)
	}
	res, err := p.Check(flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "licensecheck:", err)
		os.Exit(2)
	}
	keys := make([]string, 0, len(res.Counts))
	for k := range res.Counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("checked %d components in %d SBOMs\n", res.Checked, flag.NArg())
	for _, k := range keys {
		fmt.Printf("  %5d  %s\n", res.Counts[k], k)
	}
	for _, e := range res.Excepted {
		fmt.Printf("excepted by policy: %s\n", e)
	}
	for _, e := range res.Unusedexc {
		fmt.Printf("warning: unused exception %s\n", e)
	}
	if len(res.Findings) > 0 {
		fmt.Printf("\nFAIL: %d component(s) violate the licence policy\n", len(res.Findings))
		for _, f := range res.Findings {
			fmt.Printf("  %s\t%s\t%s\t%s\t(%s)\n", f.Package, f.Version, f.Licence, f.Reason, f.SBOM)
		}
		os.Exit(1)
	}
	fmt.Println("OK")
}

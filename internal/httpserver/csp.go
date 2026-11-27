package httpserver

import (
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"regexp"
	"strings"
)

// inlineScript matches script elements without attributes. SvelteKit's static
// SPA fallback contains exactly one such bootstrap script whose body embeds a
// per-build version, so its hash changes with every build.
var inlineScript = regexp.MustCompile(`(?s)<script>(.*?)</script>`)

// contentSecurityPolicy builds the CSP for the embedded build. Script hashes
// are computed from the embedded index.html at startup, so scripts never need
// 'unsafe-inline'. Only trusted, build-time content is hashed; the server
// never hashes request data.
//
// style-src keeps 'unsafe-inline' because Svelte and xterm.js emit style
// attributes/elements; this is accepted for styles only and is not extended to
// scripts.
func contentSecurityPolicy(webFS fs.FS) string {
	scriptSrc := []string{"'self'"}
	if index, err := fs.ReadFile(webFS, "index.html"); err == nil {
		for _, m := range inlineScript.FindAllSubmatch(index, -1) {
			sum := sha256.Sum256(m[1])
			scriptSrc = append(scriptSrc, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
		}
	}
	return strings.Join([]string{
		"default-src 'none'",
		"script-src " + strings.Join(scriptSrc, " "),
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data:",
		"font-src 'self'",
		"connect-src 'self'",
		"base-uri 'none'",
		"form-action 'self'",
		"frame-ancestors 'none'",
		"object-src 'none'",
	}, "; ")
}

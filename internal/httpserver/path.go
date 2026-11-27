package httpserver

import (
	"io/fs"
	"path"
	"strings"
)

// cleanAssetPath converts a decoded request path into a name valid for an
// fs.FS rooted at the site root. It is deliberately strict: anything that could
// be a traversal or hidden file is rejected rather than normalised away.
// The empty path and "/" map to "index.html".
func cleanAssetPath(urlPath string) (string, bool) {
	if !strings.HasPrefix(urlPath, "/") {
		return "", false
	}
	if strings.ContainsAny(urlPath, "\x00\\") {
		return "", false
	}
	for _, seg := range strings.Split(urlPath, "/") {
		if seg == ".." || (strings.HasPrefix(seg, ".") && seg != "") {
			return "", false // traversal and dotfiles (.gitkeep, .well-known, ...)
		}
	}
	name := strings.TrimPrefix(path.Clean(urlPath), "/")
	if name == "" {
		name = "index.html"
	}
	if !fs.ValidPath(name) {
		return "", false
	}
	return name, true
}

// isReserved reports whether the first path segment belongs to the server and
// must never be answered with the SPA fallback.
func isReserved(urlPath string) bool {
	first := strings.TrimPrefix(urlPath, "/")
	if i := strings.IndexByte(first, '/'); i >= 0 {
		first = first[:i]
	}
	return strings.EqualFold(first, "api") || strings.EqualFold(first, "healthz")
}

// staticExtensions are file extensions that denote a static asset. A missing
// file with one of these is a real 404. Other dotted last segments (task IDs
// such as linux.facts, host aliases) are application routes.
var staticExtensions = map[string]struct{}{
	".js": {}, ".mjs": {}, ".css": {}, ".map": {}, ".json": {}, ".txt": {},
	".ico": {}, ".png": {}, ".svg": {}, ".webp": {}, ".jpg": {}, ".woff": {},
	".woff2": {}, ".webmanifest": {}, ".wasm": {}, ".html": {},
}

// isMissingAsset reports whether a name that did not resolve to a file must be a
// 404 instead of the SPA fallback: anything under _app/, or a static-asset extension.
func isMissingAsset(name string) bool {
	if name == "_app" || strings.HasPrefix(name, "_app/") {
		return true
	}
	_, ok := staticExtensions[strings.ToLower(path.Ext(name))]
	return ok
}

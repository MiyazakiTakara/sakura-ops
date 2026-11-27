// Package webui embeds the static SvelteKit build.
//
// The build output is copied into dist/ by `make web-build`; only dist/.gitkeep
// is committed, so a clean checkout still compiles and `go test ./...` works.
// httpserver answers 503 "frontend not built" when dist/index.html is absent.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the embedded build rooted at the site root (index.html at "/").
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// Cannot happen: "dist" is a compile-time embedded directory.
		panic("webui: " + err.Error())
	}
	return sub
}

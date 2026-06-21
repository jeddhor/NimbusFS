// Package web embeds the compiled frontend (web/dist, produced by `npm run build`)
// into the nimbusfs binary so it ships as a single executable.
package web

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// Dist returns the embedded frontend rooted at dist/, ready to serve.
func Dist() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

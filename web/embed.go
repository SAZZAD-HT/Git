// Package web embeds the built frontend (web/dist) into the anvil binary.
//
// `go run ./tools/devtool build` runs the Vite build first, so dist/ holds index.html and
// assets/. In a checkout where the frontend has not been built, dist/ contains only
// placeholder.html (committed), and the server shows that instead; Go code still builds
// and tests without Node.js installed.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the built frontend with dist/ stripped, so index.html is at the root.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // impossible: "dist" is a valid embedded directory
	}
	return sub
}

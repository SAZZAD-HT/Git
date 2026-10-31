// Package web embeds the built frontend (web/dist) into the anvil binary.
//
// `go run ./tools/devtool build` runs the Vite build first, so dist/ holds index.html and
// assets/. In a checkout where the frontend has not been built, dist/ contains only
// placeholder.html (committed), and the server shows that instead; Go code still builds
// and tests without Node.js installed.
package web

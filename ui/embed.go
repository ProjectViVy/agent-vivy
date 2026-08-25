//go:build !vivy_headless

// Package ui embeds the built Vite frontend into the Vivy executable.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var distFS embed.FS

// Handler serves the embedded production build.
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("ui: embedded dist missing: " + err.Error())
	}
	return NewHandler(sub)
}

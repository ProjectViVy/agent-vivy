package ui

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// NewHandler serves build assets and falls back to index.html for client routes.
func NewHandler(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(fsys, name); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				http.Error(w, "ui unavailable", http.StatusInternalServerError)
				return
			}
			name = "index.html"
		}

		if name == "index.html" {
			if _, err := fs.Stat(fsys, name); err != nil {
				http.Error(w, "UI not built: run `pnpm --dir ui install --frozen-lockfile && pnpm --dir ui build` and restart.", http.StatusServiceUnavailable)
				return
			}
			http.ServeFileFS(w, r, fsys, name)
			return
		}
		files.ServeHTTP(w, r)
	})
}

package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// HasAssets reports whether the embedded dist directory contains built frontend files.
func HasAssets() bool {
	entries, err := fs.ReadDir(distFS, "dist")
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// Handler returns an http.Handler that serves the embedded frontend assets.
// For SPA support, any request that doesn't match a static file is served index.html.
// When the dashboard was not built, the embedded placeholder is served instead.
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("web: cannot access embedded dist: " + err.Error())
	}

	_, indexErr := fs.Stat(sub, "index.html")
	if indexErr != nil {
		page, _ := fs.ReadFile(sub, "placeholder.html")
		if len(page) == 0 {
			page = []byte("<h1>logtailr</h1><p>web dashboard assets not built</p>")
		}
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(page)
		})
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		// Try to serve the exact file
		if path != "" {
			if _, err := fs.Stat(sub, path); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// SPA fallback: serve index.html for all non-file routes
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

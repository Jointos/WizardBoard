// Package webui serves the built Phaser client from an embedded filesystem.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler serves web/dist with a single-page-app fallback to index.html.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		// The HTML must not be cached, otherwise a restarted server can be
		// masked by an old bundle reference. Hashed assets keep default caching.
		if name == "index.html" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		if _, err := fs.Stat(sub, name); err != nil {
			w.Header().Set("Cache-Control", "no-cache")
			clone := r.Clone(r.Context())
			clone.URL.Path = "/"
			fileServer.ServeHTTP(w, clone)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

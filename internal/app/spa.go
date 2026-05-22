package app

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// mountSPA serves the web frontend from STROPPY_WEB_DIR as a catch-all, falling back
// to index.html for client-side routing. When the env is unset the SPA is not served
// (the server still runs the API + proxies) — so the binary builds and runs without
// a frontend present. Embedding web/dist can replace this later behind a build tag.
func mountSPA(mux *chi.Mux) {
	dir := os.Getenv("STROPPY_WEB_DIR")
	if dir == "" {
		return
	}
	fsys := http.Dir(dir)
	fileServer := http.FileServer(fsys)
	index := filepath.Join(dir, "index.html")

	mux.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/")
		if rel == "" {
			http.ServeFile(w, r, index)
			return
		}
		if f, err := fsys.Open(rel); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// Unknown path with no matching file -> SPA entrypoint (client-side routing).
		http.ServeFile(w, r, index)
	}))
}

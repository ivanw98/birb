package httpapi

import (
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
)

func init() {
	// Alpine has no /etc/mime.types and Go's built-in table has neither extension.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
	_ = mime.AddExtensionType(".woff2", "font/woff2")
}

// newSPAHandler serves the built Vite bundle in dir: real files by path, index.html for anything else
// so client-side routes survive a hard refresh. Returns http.HandlerFunc because chi.Mux.NotFound takes one.
func newSPAHandler(dir string) http.HandlerFunc {
	root := os.DirFS(dir)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if info, err := fs.Stat(root, name); err == nil && info.Mode().IsRegular() {
			if strings.HasPrefix(name, "assets/") {
				// Vite content-hashes everything under assets/.
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				// Everything outside assets/ keeps its name across builds, so a
				// cached sw.js would pin users to a stale service worker.
				w.Header().Set("Cache-Control", "no-cache")
			}
			http.ServeFileFS(w, r, root, name)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, root, "index.html")
	}
}

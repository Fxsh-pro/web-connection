package httpserver

import (
	"net/http"
	"path/filepath"
)

func NeuterIndex(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" || path == "" {
			http.ServeFile(w, r, filepath.Join("web", "index.html"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

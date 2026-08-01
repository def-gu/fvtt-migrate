package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:panel
var panelFiles embed.FS

func Panel() (http.Handler, error) {
	sub, err := fs.Sub(panelFiles, "panel")
	if err != nil {
		return nil, err
	}
	server := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(sub, requested(r.URL.Path)); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		server.ServeHTTP(w, r)
	}), nil
}

func requested(p string) string {
	if p == "" || p == "/" {
		return "index.html"
	}
	return p[1:]
}

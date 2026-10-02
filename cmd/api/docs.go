package main

import (
	_ "embed"
	"net/http"

	"github.com/flowchartsman/swaggerui"
	"github.com/go-chi/chi/v5"
)

//go:embed openapi.yaml
var openAPISpec []byte

//go:embed demo.html
var demoPortalHTML []byte

func mountDocs(r chi.Router) {
	r.Mount("/docs", docsHandler())
	r.Get("/demo", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/demo/", http.StatusFound)
	})
	r.Get("/demo/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(demoPortalHTML)
	})
}

func docsHandler() http.Handler {
	ui := http.StripPrefix("/docs", swaggerui.Handler(openAPISpec))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/docs" {
			http.Redirect(w, r, "/docs/", http.StatusFound)
			return
		}
		ui.ServeHTTP(w, r)
	})
}

package main

import (
	_ "embed"
	"net/http"

	"github.com/flowchartsman/swaggerui"
	"github.com/go-chi/chi/v5"
)

//go:embed openapi.yaml
var openAPISpec []byte

func mountDocs(r chi.Router) {
	r.Mount("/docs", docsHandler())
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

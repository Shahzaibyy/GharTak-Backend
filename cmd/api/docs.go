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
	r.Get("/docs", docsRedirect)
	r.Mount("/docs/", swaggerui.Handler(openAPISpec))
}

func docsRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/docs/", http.StatusFound)
}

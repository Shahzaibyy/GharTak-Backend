package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocsRedirectsToSwagger(t *testing.T) {
	handler := routes(modules{})
	response := requestDocs(t, handler, "/docs")
	if response.Code != http.StatusFound {
		t.Fatalf("status = %d", response.Code)
	}
	if loc := response.Header().Get("Location"); loc != "/docs/" {
		t.Fatalf("location = %s", loc)
	}
	page := requestDocs(t, handler, "/docs/")
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "swagger") {
		t.Fatalf("page status = %d body = %s", page.Code, page.Body.String())
	}
	spec := requestDocs(t, handler, "/docs/swagger_spec")
	if spec.Code != http.StatusOK || !strings.Contains(spec.Body.String(), "GharTak API") {
		t.Fatalf("spec status = %d", spec.Code)
	}
}

func requestDocs(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	handler.ServeHTTP(recorder, request)
	return recorder
}

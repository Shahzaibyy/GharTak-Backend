package httpserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/yourusername/ghartak-backend/internal/platform/httpserver"
)

type fakePing struct {
	err error
}

func (f fakePing) Ping(context.Context) error { return f.err }

func TestHealth(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "up", status: http.StatusOK},
		{name: "down", err: errors.New("db"), status: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := httpserver.Health(fakePing{err: tt.err}, zerolog.Nop())
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Fatalf("status = %d body %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		remote string
		want   string
	}{
		{remote: "10.0.0.8:4000", want: "10.0.0.8"},
		{remote: "[::1]:5000", want: "::1"},
		{remote: "not-a-host", want: "not-a-host"},
	}
	for _, tt := range tests {
		if got := httpserver.ClientIP(tt.remote); got != tt.want {
			t.Fatalf("ClientIP(%s) = %s", tt.remote, got)
		}
	}
}

func TestRateLimit(t *testing.T) {
	limiter := httpserver.NewLimiter(1, time.Minute)
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	first := httptest.NewRequest(http.MethodGet, "/zones", nil)
	first.RemoteAddr = "10.1.1.1:1000"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, first)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("first = %d", rec.Code)
	}
	second := httptest.NewRequest(http.MethodGet, "/zones", nil)
	second.RemoteAddr = "10.1.1.1:1001"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, second)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second = %d", rec.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "rate_limited" {
		t.Fatalf("code = %s", body.Error.Code)
	}
}

func TestAllowWindow(t *testing.T) {
	limiter := httpserver.NewLimiter(1, time.Minute)
	now := time.Now()
	if !limiter.Allow("a", now) {
		t.Fatal("first")
	}
	if limiter.Allow("a", now.Add(time.Second)) {
		t.Fatal("second")
	}
	if !limiter.Allow("a", now.Add(time.Minute+time.Second)) {
		t.Fatal("after window")
	}
}

package httpserver

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/yourusername/ghartak-backend/internal/platform/apperror"
	"github.com/yourusername/ghartak-backend/internal/platform/httpx"
)

type Limiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func NewLimiter(limit int, window time.Duration) *Limiter {
	if limit < 1 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &Limiter{
		hits:   make(map[string][]time.Time),
		limit:  limit,
		window: window,
	}
}

func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if l.Allow(ClientIP(r.RemoteAddr), time.Now()) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Retry-After", strconv.Itoa(int(l.window.Seconds())))
		httpx.WriteError(w, apperror.ErrRateLimited)
	})
}

func (l *Limiter) Allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweepIfLarge(now)
	kept := l.recent(key, now)
	if len(kept) >= l.limit {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

func (l *Limiter) recent(key string, now time.Time) []time.Time {
	cutoff := now.Add(-l.window)
	kept := l.hits[key][:0]
	for _, ts := range l.hits[key] {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	return kept
}

func (l *Limiter) sweepIfLarge(now time.Time) {
	if len(l.hits) < 10000 {
		return
	}
	cutoff := now.Add(-l.window)
	for key, hits := range l.hits {
		if !hasRecent(hits, cutoff) {
			delete(l.hits, key)
		}
	}
}

func hasRecent(hits []time.Time, cutoff time.Time) bool {
	for _, ts := range hits {
		if ts.After(cutoff) {
			return true
		}
	}
	return false
}

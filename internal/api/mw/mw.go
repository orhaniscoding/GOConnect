package mw

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Chain composes middlewares from left to right.
func Chain(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// BearerAuth enforces Authorization: Bearer <token>. Empty token disables auth.
func BearerAuth(tokenFn func() string, logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := strings.TrimSpace(tokenFn())
			if tok == "" {
				next.ServeHTTP(w, r)
				return
			}
			ah := r.Header.Get("Authorization")
			if ah == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code":"unauthorized","message":"missing bearer token"}`))
				return
			}
			parts := strings.SplitN(ah, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code":"unauthorized","message":"invalid auth scheme"}`))
				return
			}
			if parts[1] != tok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"code":"forbidden","message":"invalid token"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit applies token-bucket rate limiting per client key.
// Key is remote IP combined with optional subject derived from Authorization header.
func RateLimit(rps, burst int, logger *log.Logger) func(http.Handler) http.Handler {
	if rps <= 0 {
		rps = 10
	}
	if burst <= 0 {
		burst = 20
	}
	type entry struct {
		lim  *rate.Limiter
		last time.Time
	}
	var (
		mu sync.Mutex
		m  = map[string]*entry{}
	)
	gc := func() {
		mu.Lock()
		defer mu.Unlock()
		cutoff := time.Now().Add(-5 * time.Minute)
		for k, e := range m {
			if e.last.Before(cutoff) {
				delete(m, k)
			}
		}
	}
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			gc()
		}
	}()

	getKey := func(r *http.Request) string {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		subj := ""
		if ah := r.Header.Get("Authorization"); ah != "" {
			// we don't validate; only attach suffix for bucketing
			subj = ah
		}
		if subj != "" {
			return host + "|" + subj
		}
		return host
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := getKey(r)
			mu.Lock()
			e := m[key]
			if e == nil {
				e = &entry{lim: rate.NewLimiter(rate.Limit(rps), burst)}
				m[key] = e
			}
			e.last = time.Now()
			mu.Unlock()

			if !e.lim.Allow() {
				// approximate retry-after based on time to single token
				ra := time.Duration(float64(time.Second) / float64(rps))
				w.Header().Set("Retry-After", strconv.Itoa(int(ra.Seconds())))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"code":"rate_limited","message":"too many requests"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

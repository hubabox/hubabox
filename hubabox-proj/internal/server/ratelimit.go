package server

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// authPostLimiter implements a sliding window: at most max timestamps per key within window.
type authPostLimiter struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	attempts map[string][]time.Time
}

func newAuthPostLimiter(max int, window time.Duration) *authPostLimiter {
	return &authPostLimiter{
		max:      max,
		window:   window,
		attempts: make(map[string][]time.Time),
	}
}

func (l *authPostLimiter) allow(key string) bool {
	now := time.Now()
	cut := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	slice := l.attempts[key]
	kept := slice[:0]
	for _, t := range slice {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.attempts[key] = kept
		return false
	}
	kept = append(kept, now)
	l.attempts[key] = kept
	return true
}

func rateLimitKey(routeKey string, r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	return routeKey + "|" + ip
}

// rateLimitRepeatedPost limits requests per client IP (use after middleware.RealIP).
func (s *Server) rateLimitRepeatedPost(lim *authPostLimiter, routeKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !lim.allow(rateLimitKey(routeKey, r)) {
				s.log.Warn("rate limit", "route", routeKey, "remote", r.RemoteAddr)
				w.Header().Set("Retry-After", "60")
				http.Error(w, "Too many attempts. Try again in about a minute.", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

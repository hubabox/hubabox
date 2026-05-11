package server

import (
	"net/http"
	"testing"
	"time"
)

func TestAuthPostLimiter(t *testing.T) {
	l := newAuthPostLimiter(3, 500*time.Millisecond)
	key := "login|192.0.2.1"
	if !l.allow(key) || !l.allow(key) || !l.allow(key) {
		t.Fatal("expected first 3 attempts to succeed")
	}
	if l.allow(key) {
		t.Fatal("4th attempt in window should fail")
	}
	time.Sleep(550 * time.Millisecond)
	if !l.allow(key) {
		t.Fatal("after window, attempt should succeed")
	}
}

func TestRateLimitKey(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.7:12345"
	if got := rateLimitKey("setup", r); got != "setup|203.0.113.7" {
		t.Fatalf("got %q", got)
	}
}

package server

import (
	"net/http/httptest"
	"testing"
)

func TestNormalizePublicOrigin(t *testing.T) {
	if got := normalizePublicOrigin("http://10.0.0.5:8787/"); got != "http://10.0.0.5:8787" {
		t.Fatalf("got %q", got)
	}
	if got := normalizePublicOrigin("192.168.1.2:9999"); got != "http://192.168.1.2:9999" {
		t.Fatalf("got %q", got)
	}
	if normalizePublicOrigin("ftp://x") != "" {
		t.Fatal("want reject non-http(s)")
	}
}

func TestLibraryInviteOrigin_LocalhostUsesLAN(t *testing.T) {
	r := httptest.NewRequest("GET", "http://127.0.0.1:8787/files", nil)
	r.Host = "127.0.0.1:8787"
	if got := libraryInviteOrigin(r, []string{"192.168.0.7"}, ":8787", "other", ""); got != "http://192.168.0.7:8787" {
		t.Fatalf("got %q want http://192.168.0.7:8787", got)
	}
}

// Browsers send Host: localhost:8787 when you open admin at localhost; that name is not net.ParseIP,
// so we must still fall through to LAN (same bug users hit with the LAN card showing a real IP).
func TestLibraryInviteOrigin_LocalhostHostnameUsesLAN(t *testing.T) {
	r := httptest.NewRequest("GET", "http://localhost:8787/files", nil)
	r.Host = "localhost:8787"
	if got := libraryInviteOrigin(r, []string{"192.168.0.7"}, ":8787", "other", ""); got != "http://192.168.0.7:8787" {
		t.Fatalf("got %q want http://192.168.0.7:8787", got)
	}
}

func TestLibraryInviteOrigin_LocalhostUsesHostnameLocal(t *testing.T) {
	r := httptest.NewRequest("GET", "http://127.0.0.1:8787/files", nil)
	r.Host = "127.0.0.1:8787"
	if got := libraryInviteOrigin(r, nil, ":8787", "pop-os", ""); got != "http://pop-os.local:8787" {
		t.Fatalf("got %q want http://pop-os.local:8787", got)
	}
}

func TestLibraryInviteOrigin_PublicOriginWins(t *testing.T) {
	r := httptest.NewRequest("GET", "http://127.0.0.1:8787/files", nil)
	r.Host = "127.0.0.1:8787"
	if got := libraryInviteOrigin(r, []string{"10.0.0.1"}, ":8787", "x", "http://192.168.99.1:9000"); got != "http://192.168.99.1:9000" {
		t.Fatalf("got %q", got)
	}
}

func TestLibraryInviteOrigin_UsesRequestHost(t *testing.T) {
	r := httptest.NewRequest("GET", "http://192.168.0.7:8787/files", nil)
	r.Host = "192.168.0.7:8787"
	if got := libraryInviteOrigin(r, []string{"10.0.0.1"}, ":8787", "x", ""); got != "http://192.168.0.7:8787" {
		t.Fatalf("got %q", got)
	}
}

func TestLibraryInviteOrigin_NoLANNoLoop(t *testing.T) {
	r := httptest.NewRequest("GET", "http://192.168.0.7:8787/files", nil)
	r.Host = "192.168.0.7:8787"
	if got := libraryInviteOrigin(r, nil, ":8787", "x", ""); got != "http://192.168.0.7:8787" {
		t.Fatalf("got %q", got)
	}
}

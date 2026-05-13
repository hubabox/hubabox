package server

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kros/hubabox/internal/config"
	"github.com/kros/hubabox/internal/db"
)

func newTestHTTPServer(t *testing.T) (baseURL string, cleanup func()) {
	t.Helper()
	ctx := context.Background()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "files"), 0o750); err != nil {
		t.Fatal(err)
	}
	openDB, err := db.Open(ctx, filepath.Join(tmp, "hubabox.db"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.ForTest(tmp)
	srv, err := New(cfg, openDB)
	if err != nil {
		_ = openDB.Close()
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	return ts.URL, func() {
		ts.Close()
		_ = openDB.Close()
	}
}

func TestHTTPHealth(t *testing.T) {
	baseURL, cleanup := newTestHTTPServer(t)
	defer cleanup()

	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "ok" {
		t.Fatalf("body %q", b)
	}
}

func clientNoRedirect(jar http.CookieJar) *http.Client {
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func TestHTTPRootRedirectsToSetupWhenNoAdmin(t *testing.T) {
	baseURL, cleanup := newTestHTTPServer(t)
	defer cleanup()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := clientNoRedirect(jar).Get(baseURL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status %d want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != "/setup" {
		t.Fatalf("Location %q", loc)
	}
}

func TestHTTPSetupPostThenFilesRequiresSession(t *testing.T) {
	baseURL, cleanup := newTestHTTPServer(t)
	defer cleanup()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := clientNoRedirect(jar)

	resp, err := client.PostForm(baseURL+"/setup", url.Values{
		"password":  []string{"abcdefghijklmnop"},
		"password2": []string{"abcdefghijklmnop"},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("setup status %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/files" {
		t.Fatalf("Location %q want /files", loc)
	}

	clientFollow := &http.Client{Jar: jar}
	resp2, err := clientFollow.Get(baseURL + "/files")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("/files status %d", resp2.StatusCode)
	}
	body, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(body), "hubaBox") || !strings.Contains(string(body), "Drop files") {
		snippet := string(body)
		if len(snippet) > 400 {
			snippet = snippet[:400] + "..."
		}
		t.Fatalf("unexpected /files body: %q", snippet)
	}
}

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

func parseLibraryInviteTokenFromFilesHTML(html string) (string, bool) {
	const prefix = `data-invite-token="`
	i := strings.Index(html, prefix)
	if i < 0 {
		return "", false
	}
	i += len(prefix)
	j := strings.Index(html[i:], `"`)
	if j < 0 {
		return "", false
	}
	return html[i : i+j], true
}

func newTestHTTPServer(t *testing.T) (baseURL, dataDir string, cleanup func()) {
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
	return ts.URL, tmp, func() {
		ts.Close()
		_ = openDB.Close()
	}
}

func TestHTTPHealth(t *testing.T) {
	baseURL, _, cleanup := newTestHTTPServer(t)
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
	baseURL, _, cleanup := newTestHTTPServer(t)
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
	baseURL, _, cleanup := newTestHTTPServer(t)
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
	if !strings.Contains(string(body), "hubaBox") || !strings.Contains(string(body), "Drop files") || !strings.Contains(string(body), "Hub storage") {
		snippet := string(body)
		if len(snippet) > 400 {
			snippet = snippet[:400] + "..."
		}
		t.Fatalf("unexpected /files body: %q", snippet)
	}
}

func TestHTTPFilesPreviewInlineWhenAuthed(t *testing.T) {
	baseURL, dataDir, cleanup := newTestHTTPServer(t)
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

	notePath := filepath.Join(dataDir, "files", "note.txt")
	if err := os.WriteFile(notePath, []byte("hello preview"), 0o644); err != nil {
		t.Fatal(err)
	}

	clientFollow := &http.Client{Jar: jar}
	prev, err := clientFollow.Get(baseURL + "/files/preview/note.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer prev.Body.Close()
	if prev.StatusCode != http.StatusOK {
		t.Fatalf("preview status %d", prev.StatusCode)
	}
	if ct := prev.Header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type %q want text/plain; charset=utf-8", ct)
	}
	if cd := prev.Header.Get("Content-Disposition"); !strings.Contains(cd, "inline") {
		t.Fatalf("Content-Disposition %q want inline", cd)
	}
	body, _ := io.ReadAll(prev.Body)
	if string(body) != "hello preview" {
		t.Fatalf("body %q", body)
	}

	bad, err := clientFollow.Get(baseURL + "/files/preview/nope.exe")
	if err != nil {
		t.Fatal(err)
	}
	bad.Body.Close()
	if bad.StatusCode != http.StatusNotFound {
		t.Fatalf("non-previewable status %d want 404", bad.StatusCode)
	}

	noFollow := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	noSess, err := noFollow.Get(baseURL + "/files/preview/note.txt")
	if err != nil {
		t.Fatal(err)
	}
	noSess.Body.Close()
	if noSess.StatusCode != http.StatusSeeOther {
		t.Fatalf("unauthenticated preview status %d want 303", noSess.StatusCode)
	}
	if loc := noSess.Header.Get("Location"); loc != "/login" {
		t.Fatalf("Location %q want /login", loc)
	}

	ins, err := clientFollow.Get(baseURL + "/files/insight/fragment/note.txt")
	if err != nil {
		t.Fatal(err)
	}
	bodyIns, _ := io.ReadAll(ins.Body)
	ins.Body.Close()
	if ins.StatusCode != http.StatusOK {
		t.Fatalf("insight status %d body %s", ins.StatusCode, string(bodyIns))
	}
	sb := string(bodyIns)
	if !strings.Contains(sb, "Read-only summary") || !strings.Contains(sb, "text/plain") {
		if len(sb) > 500 {
			sb = sb[:500] + "..."
		}
		t.Fatalf("unexpected insight page: %q", sb)
	}
}

func TestHTTPLibraryPreviewWhenGuestUnlocked(t *testing.T) {
	baseURL, dataDir, cleanup := newTestHTTPServer(t)
	defer cleanup()

	adminJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	adminNoRedir := clientNoRedirect(adminJar)
	resp, err := adminNoRedir.PostForm(baseURL+"/setup", url.Values{
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

	en, err := adminNoRedir.PostForm(baseURL+"/files/library/enable", url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	en.Body.Close()
	if en.StatusCode != http.StatusSeeOther {
		t.Fatalf("library enable status %d", en.StatusCode)
	}

	adminFollow := &http.Client{Jar: adminJar}
	filesPage, err := adminFollow.Get(baseURL + "/files")
	if err != nil {
		t.Fatal(err)
	}
	bodyFiles, _ := io.ReadAll(filesPage.Body)
	filesPage.Body.Close()
	if filesPage.StatusCode != http.StatusOK {
		t.Fatalf("/files status %d", filesPage.StatusCode)
	}
	libTok, ok := parseLibraryInviteTokenFromFilesHTML(string(bodyFiles))
	if !ok || libTok == "" {
		t.Fatal("could not parse library token from /files HTML")
	}

	guestPath := filepath.Join(dataDir, "files", "guest.txt")
	if err := os.WriteFile(guestPath, []byte("library preview"), 0o644); err != nil {
		t.Fatal(err)
	}

	guestJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	guestNoRedir := clientNoRedirect(guestJar)
	unl, err := guestNoRedir.PostForm(baseURL+"/library/unlock", url.Values{
		"display_name": []string{"TestGuest"},
		"token":        []string{libTok},
	})
	if err != nil {
		t.Fatal(err)
	}
	unl.Body.Close()
	if unl.StatusCode != http.StatusSeeOther {
		t.Fatalf("library unlock status %d want 303", unl.StatusCode)
	}
	if loc := unl.Header.Get("Location"); loc != "/library" {
		t.Fatalf("unlock Location %q want /library", loc)
	}

	guest := &http.Client{Jar: guestJar}
	prev, err := guest.Get(baseURL + "/library/preview/guest.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer prev.Body.Close()
	if prev.StatusCode != http.StatusOK {
		t.Fatalf("library preview status %d", prev.StatusCode)
	}
	if !strings.Contains(prev.Header.Get("Content-Disposition"), "inline") {
		t.Fatalf("Content-Disposition %q", prev.Header.Get("Content-Disposition"))
	}
	b, _ := io.ReadAll(prev.Body)
	if string(b) != "library preview" {
		t.Fatalf("body %q", b)
	}

	bad, err := guest.Get(baseURL + "/library/preview/nope.exe")
	if err != nil {
		t.Fatal(err)
	}
	bad.Body.Close()
	if bad.StatusCode != http.StatusNotFound {
		t.Fatalf("non-previewable library preview %d want 404", bad.StatusCode)
	}

	noGuest := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	forbidden, err := noGuest.Get(baseURL + "/library/preview/guest.txt")
	if err != nil {
		t.Fatal(err)
	}
	forbidden.Body.Close()
	if forbidden.StatusCode != http.StatusForbidden {
		t.Fatalf("unauthenticated library preview status %d want 403", forbidden.StatusCode)
	}

	frag, err := guest.Get(baseURL + "/library/chat/fragment")
	if err != nil {
		t.Fatal(err)
	}
	defer frag.Body.Close()
	if frag.StatusCode != http.StatusOK {
		t.Fatalf("library chat fragment status %d", frag.StatusCode)
	}
	fragBody, _ := io.ReadAll(frag.Body)
	if !strings.Contains(string(fragBody), "lib-chat-msgs") {
		t.Fatalf("fragment missing ul: %q", string(fragBody)[:min(300, len(fragBody))])
	}
}

func TestHTTPLibraryChatFragmentForbiddenWithoutGuest(t *testing.T) {
	baseURL, _, cleanup := newTestHTTPServer(t)
	defer cleanup()
	resp, err := http.Get(baseURL + "/library/chat/fragment")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("chat fragment without guest status %d want 403", resp.StatusCode)
	}
}

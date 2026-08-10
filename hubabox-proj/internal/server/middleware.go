package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/kros/hubabox/internal/auth"
	"github.com/kros/hubabox/internal/library"
)

const csrfCookie = "hubabox_csrf"

// csrfToken returns a per-browser random token. It is intentionally separate
// from authentication cookies: the form field must be readable by the HTML,
// while the cookie provides the same-origin proof.
func (s *Server) csrfToken(w http.ResponseWriter, r *http.Request) string {
	if r != nil {
		if c, err := r.Cookie(csrfCookie); err == nil && len(c.Value) >= 32 {
			return c.Value
		}
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{Name: csrfCookie, Value: token, Path: "/", MaxAge: 14 * 24 * 3600, HttpOnly: false, SameSite: http.SameSiteStrictMode})
	return token
}

func (s *Server) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(csrfCookie)
		if err != nil || cookie.Value == "" {
			http.Error(w, "Invalid or missing security token. Refresh the page and try again.", http.StatusForbidden)
			return
		}
		// Scripted multipart uploads send the token in a header. This avoids
		// pre-parsing (and temporarily spooling) multi-gigabyte bodies before the
		// upload handler can stream them straight to hub storage.
		formToken := r.Header.Get("X-HubaBox-CSRF")
		if formToken == "" {
			parseErr := r.ParseForm()
			if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				parseErr = r.ParseMultipartForm(8 << 20)
			}
			if parseErr != nil {
				http.Error(w, "Invalid or missing security token. Refresh the page and try again.", http.StatusForbidden)
				return
			}
			formToken = r.FormValue("csrf_token")
		}
		if len(formToken) != len(cookie.Value) || subtle.ConstantTimeCompare([]byte(formToken), []byte(cookie.Value)) != 1 {
			http.Error(w, "Invalid or missing security token. Refresh the page and try again.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "geolocation=(), camera=(), payment=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; img-src 'self' data:; media-src 'self'; style-src 'self'; script-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

const (
	sessionCookie             = "hubabox_session"
	libraryCookie             = "hubabox_library"
	libraryNickCookie         = "hubabox_library_nick"
	libraryFileListSeenCookie = "hubabox_library_seen"
	libraryMaxAge             = 30 * 24 * 3600
	librarySeenMaxAge         = 400 * 24 * 3600
)

func (s *Server) sessionToken(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := s.sessionToken(r)
		ok, err := auth.ValidateSession(r.Context(), s.db, tok)
		if err != nil {
			s.log.Error("session validate", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) libraryToken(r *http.Request) string {
	c, err := r.Cookie(libraryCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

func (s *Server) setLibraryCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     libraryCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   libraryMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearLibraryCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     libraryCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) libraryGuestNick(r *http.Request) string {
	c, err := r.Cookie(libraryNickCookie)
	if err != nil || c.Value == "" {
		return ""
	}
	nick, err := library.NormalizeDisplayNick(c.Value)
	if err != nil {
		return ""
	}
	return nick
}

func (s *Server) setLibraryNickCookie(w http.ResponseWriter, nick string) {
	http.SetCookie(w, &http.Cookie{
		Name:     libraryNickCookie,
		Value:    nick,
		Path:     "/",
		MaxAge:   libraryMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearLibraryNickCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     libraryNickCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearLibraryGuestCookies clears library token and display-name cookies.
func (s *Server) clearLibraryGuestCookies(w http.ResponseWriter) {
	s.clearLibraryCookie(w)
	s.clearLibraryNickCookie(w)
	s.clearLibraryFileListSeenCookie(w)
}

func (s *Server) clearLibraryFileListSeenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     libraryFileListSeenCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) libraryFileListSeenAt(r *http.Request) (t time.Time, ok bool) {
	c, err := r.Cookie(libraryFileListSeenCookie)
	if err != nil || strings.TrimSpace(c.Value) == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, c.Value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func (s *Server) setLibraryFileListSeenCookie(w http.ResponseWriter, at time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     libraryFileListSeenCookie,
		Value:    at.UTC().Format(time.RFC3339Nano),
		Path:     "/",
		MaxAge:   librarySeenMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) requireLibraryGuest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, err := library.ValidGuest(r.Context(), s.db, s.libraryToken(r))
		if err != nil {
			s.log.Error("library guest", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !ok {
			if r.Method == http.MethodPost {
				http.Redirect(w, r, "/library", http.StatusSeeOther)
				return
			}
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string, maxAge int) {
	if maxAge <= 0 {
		maxAge = 14 * 24 * 3600
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

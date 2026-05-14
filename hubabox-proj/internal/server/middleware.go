package server

import (
	"net/http"

	"github.com/kros/hubabox/internal/auth"
	"github.com/kros/hubabox/internal/library"
)

const (
	sessionCookie     = "hubabox_session"
	libraryCookie     = "hubabox_library"
	libraryNickCookie = "hubabox_library_nick"
	libraryMaxAge     = 30 * 24 * 3600
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

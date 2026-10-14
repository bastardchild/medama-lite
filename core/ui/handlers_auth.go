package ui

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/medama-io/medama/model"
	"github.com/medama-io/medama/util/logger"
)

// expiredCookieMaxAge instructs the browser to drop the cookie immediately.
const expiredCookieMaxAge = -1

// handleLoginPage renders the sign-in form, or redirects home when a valid
// session already exists.
func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.sessionUserID(r); ok {
		redirect(w, r, "/")

		return
	}

	s.renderLogin(w, r, http.StatusOK, "")
}

// renderLogin renders the login page with an optional error message.
func (s *Server) renderLogin(w http.ResponseWriter, r *http.Request, status int, message string) {
	view := LoginView{
		Base: Base{
			Title:   "Sign in",
			Version: s.version,
		},
		Error: message,
	}

	if err := s.renderer.RenderPage(w, status, "page_login", &view); err != nil {
		http.Error(w, "failed to render login page", http.StatusInternalServerError)
	}
}

// handleLoginSubmit validates credentials against the SQLite users table and
// establishes a session cookie.
//
// This mirrors services.Handler.PostAuthLogin but is implemented directly so the
// cookie's Secure attribute can follow the actual request scheme: browsers
// silently drop Secure cookies served over plain HTTP, which is the common case
// when MedamaLite runs on 127.0.0.1:8080.
func (s *Server) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()

	if err := r.ParseForm(); err != nil {
		s.renderLogin(w, r, http.StatusBadRequest, "The sign-in form could not be read.")

		return
	}

	username := strings.TrimSpace(r.PostFormValue("username"))
	password := r.PostFormValue("password")

	if username == "" || password == "" {
		s.renderLogin(w, r, http.StatusBadRequest, "Enter both your username and password.")

		return
	}

	user, err := s.appDB.GetUserByUsername(r.Context(), username)
	if err != nil {
		log.Debug().Err(err).Str("username", username).Msg("ui: sign in failed, unknown user")
		s.renderLogin(w, r, http.StatusUnauthorized, "Incorrect username or password.")

		return
	}

	match, err := s.auth.ComparePasswords(password, user.Password)
	if err != nil {
		log.Error().Err(err).Msg("ui: failed to compare password hashes")
		s.renderLogin(w, r, http.StatusInternalServerError, "Sign in is temporarily unavailable.")

		return
	}

	if !match {
		log.Debug().Str("username", username).Msg("ui: sign in failed, password mismatch")
		s.renderLogin(w, r, http.StatusUnauthorized, "Incorrect username or password.")

		return
	}

	cookie, err := s.auth.CreateSession(r.Context(), user.ID)
	if err != nil {
		log.Error().Err(err).Msg("ui: failed to create session")
		s.renderLogin(w, r, http.StatusInternalServerError, "Could not start a session. Please try again.")

		return
	}

	cookie.Secure = isSecureRequest(r)
	http.SetCookie(w, cookie)

	log.Info().Str("username", user.Username).Msg("ui: user signed in")
	redirect(w, r, "/")
}

// handleLogout revokes the session and clears the cookie.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(model.SessionCookieName); err == nil && cookie.Value != "" {
		s.revokeSession(r, cookie.Value)
	}

	// Expire the cookie in the browser.
	http.SetCookie(w, &http.Cookie{
		Name:     model.SessionCookieName,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   expiredCookieMaxAge,
		Expires:  time.Unix(0, 0),
	})

	redirect(w, r, "/login")
}

// revokeSession removes a session from the in-memory cache.
//
// The cookie holds the base64 encoded, AES-GCM encrypted session id, while the
// cache is keyed by the plain session id, so the value must be decoded and
// decrypted before it can be revoked.
func (s *Server) revokeSession(r *http.Request, cookieValue string) {
	decoded, err := base64.URLEncoding.DecodeString(cookieValue)
	if err != nil {
		return
	}

	sessionID, err := s.auth.DecryptSession(r.Context(), string(decoded))
	if err != nil {
		return
	}

	s.auth.RevokeSession(r.Context(), sessionID)
}

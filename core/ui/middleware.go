package ui

import (
	"context"
	"net/http"
	"strings"

	"github.com/medama-io/medama/model"
)

const (
	// htmxRequestHeader marks a request originating from HTMX.
	htmxRequestHeader = "HX-Request"
	// htmxRedirectHeader asks HTMX to perform a client-side redirect.
	htmxRedirectHeader = "HX-Redirect"
	// htmxRetargetHeader changes the swap target of a response.
	htmxRetargetHeader = "HX-Retarget"
	// htmxReswapHeader changes how a response is swapped in.
	htmxReswapHeader = "HX-Reswap"
)

// isHTMX reports whether the request was issued by HTMX.
func isHTMX(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get(htmxRequestHeader), "true")
}

// requireSession rejects unauthenticated requests, redirecting browsers to the
// login page and instructing HTMX to do the same client-side.
func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := s.sessionUserID(r)
		if !ok {
			redirect(w, r, "/login")

			return
		}

		ctx := context.WithValue(r.Context(), model.ContextKeyUserID, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// sessionUserID resolves the signed-in user from the session cookie.
func (s *Server) sessionUserID(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(model.SessionCookieName)
	if err != nil || cookie.Value == "" {
		return "", false
	}

	userID, err := s.auth.ReadSession(r.Context(), cookie.Value)
	if err != nil {
		return "", false
	}

	return userID, true
}

// currentUserID returns the user id stored in the request context by
// requireSession. It is empty for unauthenticated requests.
func currentUserID(ctx context.Context) string {
	userID, _ := ctx.Value(model.ContextKeyUserID).(string)

	return userID
}

// redirect sends the client to url, using the HTMX redirect header when the
// request came from HTMX so the swap does not try to render the redirect body.
func redirect(w http.ResponseWriter, r *http.Request, url string) {
	if isHTMX(r) {
		w.Header().Set(htmxRedirectHeader, url)
		w.WriteHeader(http.StatusOK)

		return
	}

	http.Redirect(w, r, url, http.StatusSeeOther)
}

// isSecureRequest reports whether the original request was served over HTTPS.
// It is used to decide whether session cookies may carry the Secure attribute,
// which browsers reject over plain HTTP (for example on 127.0.0.1:8080).
func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}

	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// baseFor builds the layout fields shared by every full page. It resolves the
// signed-in username from the application database; page handlers therefore do
// not need to fetch the user themselves.
func (s *Server) baseFor(r *http.Request, title string) Base {
	base := Base{
		Title:   title,
		Version: s.version,
		IsDemo:  s.auth.IsDemoMode,
	}

	if userID := currentUserID(r.Context()); userID != "" {
		user, err := s.appDB.GetUser(r.Context(), userID)
		if err == nil {
			base.Username = user.Username
		}
	}

	return base
}

// renderError renders a full error page, or an inline fragment when the request
// originated from HTMX.
func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	data := struct {
		Base
		Status  int
		Message string
	}{
		Base:    s.baseFor(r, "Error"),
		Status:  status,
		Message: message,
	}

	if isHTMX(r) {
		if err := s.renderer.Render(w, status, "partial_error", data); err != nil {
			http.Error(w, message, status)
		}

		return
	}

	if err := s.renderer.RenderPage(w, status, "page_error", &data); err != nil {
		http.Error(w, message, status)
	}
}

package ui

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/medama-io/medama/api"
	"github.com/medama-io/medama/iputils"
	"github.com/medama-io/medama/util/logger"
)

const (
	// minPasswordLength is the minimum accepted password length.
	minPasswordLength = 8
	// maxHostnameLength is the maximum accepted domain length.
	maxHostnameLength = 253
)

// handleWebsitesPartial renders just the website list fragment, used to refresh
// the home page grid without a full page load.
func (s *Server) handleWebsitesPartial(w http.ResponseWriter, r *http.Request) {
	s.renderWebsitesList(w, r, "", nil)
}

// handleCreateWebsite adds a website and returns the refreshed list.
func (s *Server) handleCreateWebsite(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()

	if err := r.ParseForm(); err != nil {
		s.renderFormError(w, "#add-website-error", "The form could not be read.")

		return
	}

	hostname, err := normalizeHostname(r.PostFormValue("hostname"))
	if err != nil {
		s.renderFormError(w, "#add-website-error", err.Error())

		return
	}

	// PostWebsites also keeps the in-memory hostname cache used by the
	// ingestion path in sync, which is why the UI delegates to it.
	res, err := s.svc.PostWebsites(r.Context(), &api.WebsiteCreate{Hostname: hostname})
	if err != nil {
		log.Error().Err(err).Str("hostname", hostname).Msg("ui: failed to create website")
		s.renderFormError(w, "#add-website-error", "The website could not be added.")

		return
	}

	if _, ok := res.(*api.WebsiteGetHeaders); !ok {
		s.renderFormError(w, "#add-website-error", apiErrorMessage(res))

		return
	}

	log.Info().Str("hostname", hostname).Msg("ui: website created")

	s.renderWebsitesList(w, r, "medama:created", map[string]string{"hostname": hostname})
}

// handleDeleteWebsite removes a website and its analytics rows, then returns the
// refreshed list.
func (s *Server) handleDeleteWebsite(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()
	hostname := r.PathValue("hostname")

	// DeleteWebsitesID verifies ownership against the signed-in user.
	res, err := s.svc.DeleteWebsitesID(
		r.Context(),
		api.DeleteWebsitesIDParams{Hostname: hostname},
	)
	if err != nil {
		log.Error().Err(err).Str("hostname", hostname).Msg("ui: failed to delete website")
		s.renderError(w, r, http.StatusInternalServerError, "The website could not be deleted.")

		return
	}

	if _, ok := res.(*api.DeleteWebsitesIDNoContent); !ok {
		s.renderError(w, r, http.StatusBadRequest, apiErrorMessage(res))

		return
	}

	log.Info().Str("hostname", hostname).Msg("ui: website deleted")

	s.renderWebsitesList(w, r, "medama:deleted", map[string]string{"hostname": hostname})
}

// handleUpdateProfile updates the username.
func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()

	if err := r.ParseForm(); err != nil {
		s.renderFormError(w, "#profile-feedback", "The form could not be read.")

		return
	}

	username := strings.TrimSpace(r.PostFormValue("username"))
	if username == "" {
		s.renderFormError(w, "#profile-feedback", "Enter a username.")

		return
	}

	res, err := s.svc.PatchUser(
		r.Context(),
		&api.UserPatch{Username: api.NewOptString(username)},
		api.PatchUserParams{},
	)
	if err != nil {
		log.Error().Err(err).Msg("ui: failed to update profile")
		s.renderFormError(w, "#profile-feedback", "Your profile could not be updated.")

		return
	}

	if _, ok := res.(*api.UserGetHeaders); !ok {
		s.renderFormError(w, "#profile-feedback", apiErrorMessage(res))

		return
	}

	s.renderFeedback(w, "#profile-feedback", true, "Profile updated.")
}

// handleUpdatePassword updates the account password.
func (s *Server) handleUpdatePassword(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()

	if err := r.ParseForm(); err != nil {
		s.renderFormError(w, "#password-feedback", "The form could not be read.")

		return
	}

	password := r.PostFormValue("password")
	confirmation := r.PostFormValue("confirmPassword")

	switch {
	case len(password) < minPasswordLength:
		s.renderFormError(
			w,
			"#password-feedback",
			"Choose a password of at least 8 characters.",
		)

		return
	case password != confirmation:
		s.renderFormError(w, "#password-feedback", "The passwords do not match.")

		return
	}

	res, err := s.svc.PatchUser(
		r.Context(),
		&api.UserPatch{Password: api.NewOptString(password)},
		api.PatchUserParams{},
	)
	if err != nil {
		log.Error().Err(err).Msg("ui: failed to update password")
		s.renderFormError(w, "#password-feedback", "Your password could not be updated.")

		return
	}

	if _, ok := res.(*api.UserGetHeaders); !ok {
		s.renderFormError(w, "#password-feedback", apiErrorMessage(res))

		return
	}

	log.Info().Msg("ui: password updated")
	s.renderFeedback(w, "#password-feedback", true, "Password updated.")
}

// handleUpdateSystem updates the tenant-wide tracker settings.
func (s *Server) handleUpdateSystem(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()

	if err := r.ParseForm(); err != nil {
		s.renderFormError(w, "#system-feedback", "The form could not be read.")

		return
	}

	blockedIPs, err := iputils.GetAddrList(r.PostFormValue("blockedIPs"))
	if err != nil {
		s.renderFormError(w, "#system-feedback", "One or more IP addresses could not be parsed.")

		return
	}

	scriptTypes := r.PostForm["scriptType"]

	items := make([]api.TenantSettingsScriptTypeItem, 0, len(scriptTypes))
	for _, scriptType := range scriptTypes {
		if scriptType == "" {
			continue
		}

		items = append(items, api.TenantSettingsScriptTypeItem(scriptType))
	}

	// The tracker always needs at least the base script.
	if len(items) == 0 {
		items = append(items, api.TenantSettingsScriptTypeItemDefault)
	}

	settings := api.TenantSettings{
		ScriptType:        items,
		BlockAbusiveIPs:   api.NewOptBool(r.PostFormValue("blockAbusiveIPs") != ""),
		BlockTorExitNodes: api.NewOptBool(r.PostFormValue("blockTorExitNodes") != ""),
		BlockedIPs:        blockedIPs,
	}

	res, err := s.svc.PatchTenantSettings(
		r.Context(),
		&settings,
		api.PatchTenantSettingsParams{},
	)
	if err != nil {
		log.Error().Err(err).Msg("ui: failed to update system settings")
		s.renderFormError(w, "#system-feedback", "The system settings could not be saved.")

		return
	}

	if _, ok := res.(*api.TenantSettingsHeaders); !ok {
		s.renderFormError(w, "#system-feedback", apiErrorMessage(res))

		return
	}

	log.Info().Msg("ui: tenant settings updated")
	s.renderFeedback(w, "#system-feedback", true, "System settings saved.")
}

// renderWebsitesList renders the website grid and optionally dispatches a
// client-side event carrying the affected hostname.
func (s *Server) renderWebsitesList(
	w http.ResponseWriter,
	r *http.Request,
	event string,
	detail map[string]string,
) {
	log := logger.Get()

	websites, err := s.listWebsites(r)
	if err != nil {
		log.Error().Err(err).Msg("ui: failed to list websites")
		s.renderError(w, r, http.StatusInternalServerError, "Your websites could not be loaded.")

		return
	}

	if event != "" {
		setTrigger(w, event, detail)
	}

	view := HomeView{Base: s.baseFor(r, "Websites"), Websites: websites}

	if err := s.renderer.Render(w, http.StatusOK, "partial_websites", view); err != nil {
		log.Error().Err(err).Msg("ui: failed to render websites partial")
	}
}

// renderFormError renders a validation message into the given target element.
func (s *Server) renderFormError(w http.ResponseWriter, target, message string) {
	w.Header().Set(htmxRetargetHeader, target)
	w.Header().Set(htmxReswapHeader, "innerHTML")

	s.renderFeedback(w, "", false, message)
}

// renderFeedback renders a success or error message fragment.
func (s *Server) renderFeedback(w http.ResponseWriter, target string, success bool, message string) {
	log := logger.Get()

	if target != "" {
		w.Header().Set(htmxRetargetHeader, target)
		w.Header().Set(htmxReswapHeader, "innerHTML")
	}

	view := struct {
		Success bool
		Message string
	}{
		Success: success,
		Message: message,
	}

	if err := s.renderer.Render(w, http.StatusOK, "partial_feedback", view); err != nil {
		log.Error().Err(err).Msg("ui: failed to render feedback partial")
	}
}

// setTrigger sets the HX-Trigger header so HTMX dispatches a client-side event
// carrying the given detail payload.
func setTrigger(w http.ResponseWriter, event string, detail any) {
	encoded, err := json.Marshal(map[string]any{event: detail})
	if err != nil {
		return
	}

	w.Header().Set("HX-Trigger", string(encoded))
}

// normalizeHostname validates and canonicalises a domain submitted from the
// add-website form.
func normalizeHostname(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimSuffix(value, "/")

	switch {
	case value == "":
		return "", errors.New("enter a domain name")
	case strings.ContainsAny(value, "/ \t\n"):
		return "", errors.New("enter a domain name without a path")
	case len(value) > maxHostnameLength:
		return "", errors.New("that domain name is too long")
	case !strings.Contains(value, ".") && value != "localhost":
		return "", errors.New("enter a valid domain, for example example.com")
	}

	return value, nil
}

// apiErrorMessage maps an ogen error response onto a user-facing message.
func apiErrorMessage(res any) string {
	switch res.(type) {
	case *api.ConflictErrorHeaders:
		return "That resource already exists."
	case *api.NotFoundErrorHeaders:
		return "That resource could not be found."
	case *api.BadRequestErrorHeaders:
		return "The request was rejected as invalid."
	case *api.ForbiddenErrorHeaders:
		return "This action is disabled while demo mode is on."
	case *api.UnauthorisedErrorHeaders:
		return "Your session expired. Please sign in again."
	default:
		return "Something went wrong. Please try again."
	}
}

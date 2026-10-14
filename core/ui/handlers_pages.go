package ui

import (
	"context"
	"net/http"
	"time"

	"github.com/medama-io/medama/api"
	"github.com/medama-io/medama/iputils"
	"github.com/medama-io/medama/model"
	"github.com/medama-io/medama/util/logger"
)

// handleHome renders the overview of every tracked website.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()

	view := s.baseFor(r, "Websites")
	view.Active = "home"

	websites, err := s.listWebsites(r)
	if err != nil {
		log.Error().Err(err).Msg("ui: failed to list websites")
		s.renderError(w, r, http.StatusInternalServerError, "Your websites could not be loaded.")

		return
	}

	page := HomeView{Base: view, Websites: websites}

	if err := s.renderer.RenderPage(w, http.StatusOK, "page_home", &page); err != nil {
		log.Error().Err(err).Msg("ui: failed to render home page")
	}
}

// handleDashboard renders the analytics dashboard.
//
// Every panel is rendered server-side on first paint so the page is complete
// before any JavaScript runs, which also makes the dashboard usable without
// JavaScript. HTMX then refreshes individual panels as filters change.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")

	if _, ok := s.authorizeWebsite(r, hostname); !ok {
		s.renderError(w, r, http.StatusNotFound, "That website is not tracked on this account.")

		return
	}

	ctx := r.Context()
	log := logger.Get()
	partial := s.partialContextFor(r, hostname)

	view := s.baseFor(r, hostname)
	view.Active = "dashboard"
	view.Hostname = hostname

	page := DashboardView{
		Base:    view,
		Filters: partial.State,
		Periods: periodTabs(partial.State.Period),
		Snippet: Snippet(s.scriptURL(r)),
	}

	// A failing panel degrades to its empty state rather than failing the page.
	if summary, err := s.buildSummary(ctx, partial); err != nil {
		log.Error().Err(err).Str("hostname", hostname).Msg("ui: failed to build summary panel")
	} else {
		page.Summary = summary
	}

	if pages, err := s.buildPages(ctx, partial); err != nil {
		log.Error().Err(err).Str("hostname", hostname).Msg("ui: failed to build pages panel")
	} else {
		page.Pages = pages
	}

	if referrers, err := s.buildReferrers(ctx, partial); err != nil {
		log.Error().Err(err).Str("hostname", hostname).Msg("ui: failed to build referrers panel")
	} else {
		page.Referrers = referrers
	}

	if devices, err := s.buildDevices(ctx, partial); err != nil {
		log.Error().Err(err).Str("hostname", hostname).Msg("ui: failed to build devices panel")
	} else {
		page.Devices = devices
	}

	if countries, err := s.buildCountries(ctx, partial); err != nil {
		log.Error().Err(err).Str("hostname", hostname).Msg("ui: failed to build countries panel")
	} else {
		page.Countries = countries
	}

	if events, err := s.buildEvents(ctx, partial); err != nil {
		log.Error().Err(err).Str("hostname", hostname).Msg("ui: failed to build events panel")
	} else {
		page.Events = events
	}

	if err := s.renderer.RenderPage(w, http.StatusOK, "page_dashboard", &page); err != nil {
		log.Error().Err(err).Msg("ui: failed to render dashboard page")
	}
}

// handleSettingsPage renders account, system and usage settings.
func (s *Server) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.Get()

	view := s.baseFor(r, "Settings")
	view.Active = "settings"

	profile, err := s.loadProfile(ctx)
	if err != nil {
		log.Error().Err(err).Msg("ui: failed to load user profile")
	}

	usage := s.loadUsage(ctx)
	system := s.loadSystem(ctx)

	page := SettingsView{
		Base:    view,
		Profile: profile,
		Usage:   usage,
		System:  system,
	}

	if err := s.renderer.RenderPage(w, http.StatusOK, "page_settings", &page); err != nil {
		log.Error().Err(err).Msg("ui: failed to render settings page")
	}
}

// periodTabs returns the period preset buttons.
func periodTabs(active string) []Tab {
	return []Tab{
		{Label: "Today", Value: PeriodToday, Active: active == PeriodToday, Icon: "fa-solid fa-bolt"},
		{Label: "7 days", Value: Period7d, Active: active == Period7d, Icon: "fa-solid fa-calendar-week"},
		{Label: "30 days", Value: Period30d, Active: active == Period30d, Icon: "fa-solid fa-calendar-days"},
		{Label: "Custom", Value: PeriodCustom, Active: active == PeriodCustom, Icon: "fa-solid fa-sliders"},
	}
}

// listWebsites returns every website owned by the signed-in user together with
// its 24 hour visitor count.
func (s *Server) listWebsites(r *http.Request) ([]WebsiteItem, error) {
	res, err := s.svc.GetWebsites(
		r.Context(),
		api.GetWebsitesParams{Summary: api.NewOptBool(true)},
	)
	if err != nil {
		return nil, err
	}

	ok, isOK := res.(*api.GetWebsitesOKHeaders)
	if !isOK {
		return nil, nil
	}

	scriptURL := s.scriptURL(r)
	items := make([]WebsiteItem, 0, len(ok.Response))

	for _, website := range ok.Response {
		item := WebsiteItem{
			Hostname: website.Hostname,
			Snippet:  Snippet(scriptURL),
		}

		if summary, hasSummary := website.Summary.Get(); hasSummary {
			item.Visitors = summary.Visitors
		}

		items = append(items, item)
	}

	return items, nil
}

// authorizeWebsite verifies that hostname exists and belongs to the signed-in
// user, mirroring the ownership check performed by the JSON API.
func (s *Server) authorizeWebsite(r *http.Request, hostname string) (*model.Website, bool) {
	website, err := s.appDB.GetWebsite(r.Context(), hostname)
	if err != nil {
		return nil, false
	}

	if website.UserID != currentUserID(r.Context()) {
		return nil, false
	}

	return website, true
}

// scriptURL builds the absolute tracker URL used in the tracking snippet.
func (s *Server) scriptURL(r *http.Request) string {
	scheme := "http"
	if isSecureRequest(r) {
		scheme = "https"
	}

	host := r.Host
	if host == "" {
		host = "localhost:8080"
	}

	return scheme + "://" + host + "/script.js"
}

// loadProfile reads the account details for the settings page.
func (s *Server) loadProfile(ctx context.Context) (ProfileView, error) {
	res, err := s.svc.GetUser(ctx, api.GetUserParams{})
	if err != nil {
		return ProfileView{}, err
	}

	ok, isOK := res.(*api.UserGetHeaders)
	if !isOK {
		return ProfileView{}, nil
	}

	return ProfileView{
		Username:    ok.Response.Username,
		DateCreated: ok.Response.DateCreated,
		DateUpdated: ok.Response.DateUpdated,
	}, nil
}

// loadUsage reads host resource usage. Failures degrade to a zeroed view rather
// than breaking the whole page.
func (s *Server) loadUsage(ctx context.Context) UsageView {
	res, err := s.svc.GetUserUsage(ctx, api.GetUserUsageParams{})
	if err != nil {
		return UsageView{}
	}

	ok, isOK := res.(*api.UserUsageGetHeaders)
	if !isOK {
		return UsageView{}
	}

	usage := ok.Response

	return UsageView{
		CPUUsage:   usage.CPU.Usage,
		CPUCores:   usage.CPU.Cores,
		CPUThreads: usage.CPU.Threads,
		MemoryUsed: usage.Memory.Used,
		MemoryMax:  usage.Memory.Total,
		DiskUsed:   usage.Disk.Used,
		DiskMax:    usage.Disk.Total,
		MemoryPct:  ratio(usage.Memory.Used, usage.Memory.Total),
		DiskPct:    ratio(usage.Disk.Used, usage.Disk.Total),
	}
}

// loadSystem reads the tenant-wide tracker settings.
func (s *Server) loadSystem(ctx context.Context) SystemView {
	res, err := s.svc.GetTenantSettings(ctx, api.GetTenantSettingsParams{})
	if err != nil {
		return SystemView{}
	}

	ok, isOK := res.(*api.TenantSettingsHeaders)
	if !isOK {
		return SystemView{}
	}

	settings := ok.Response

	scriptTypes := make([]string, 0, len(settings.ScriptType))
	for _, item := range settings.ScriptType {
		scriptTypes = append(scriptTypes, string(item))
	}

	blockAbusive, _ := settings.BlockAbusiveIPs.Get()
	blockTor, _ := settings.BlockTorExitNodes.Get()

	return SystemView{
		ScriptType:        joinNonEmpty(scriptTypes),
		BlockAbusiveIPs:   blockAbusive,
		BlockTorExitNodes: blockTor,
		BlockedIPs:        iputils.GetAddrListString(settings.BlockedIPs),
	}
}

// ratio returns used/total clamped to the 0..1 range.
func ratio(used, total int64) float32 {
	if total <= 0 || used <= 0 {
		return 0
	}

	value := float32(used) / float32(total)
	if value > 1 {
		return 1
	}

	return value
}

// joinNonEmpty joins values with a comma, skipping empty entries.
func joinNonEmpty(values []string) string {
	result := ""

	for _, value := range values {
		if value == "" {
			continue
		}

		if result != "" {
			result += ","
		}

		result += value
	}

	return result
}

// timeNow is indirected so tests can freeze time.
//
//nolint:gochecknoglobals // Deliberate seam for deterministic tests.
var timeNow = time.Now

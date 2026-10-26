package ui

import (
	"encoding/json"
	"html/template"
)

// Base holds the fields shared by every page layout.
type Base struct {
	// Title is the document title.
	Title string
	// Content is the rendered page markup injected into the layout by
	// Renderer.RenderPage. It is never set by handlers directly.
	Content template.HTML
	// Active marks the highlighted navigation entry: home, dashboard or settings.
	Active string
	// Hostname is set on dashboard pages.
	Hostname string
	// Username is the signed-in user, shown in the navbar.
	Username string
	// Version is the server build version, shown in the footer.
	Version string
	// IsDemo is true when demo mode is enabled; mutating actions are hidden.
	IsDemo bool
}

// Layout exposes the shared layout fields. Every page view model embeds Base,
// which promotes this method, so each of them satisfies layoutView.
func (b *Base) Layout() *Base {
	return b
}

// LoginView is the view model for the login page.
type LoginView struct {
	Base
	Error string
}

// Metric is a single summary card.
type Metric struct {
	Label    string
	Value    string
	Icon     string
	Delta    string
	HasDelta bool
	// DeltaUp indicates the direction of the change, driving the arrow icon.
	DeltaUp bool
	// Good indicates whether that direction is desirable, driving the colour.
	Good bool
}

// SummaryView is the view model for the summary cards and the timeseries chart.
type SummaryView struct {
	Metrics []Metric
	Chart   ChartData
}

// ChartData is the payload handed to the client-side chart renderer.
type ChartData struct {
	Labels    []string `json:"labels"`
	Visitors  []int    `json:"visitors"`
	Pageviews []int    `json:"pageviews"`
}

// JSON serialises the chart payload for embedding into the response.
//
// encoding/json escapes '<', '>' and '&' by default, so the result can never
// terminate the surrounding <script> element.
func (c ChartData) JSON() template.JS {
	if len(c.Labels) == 0 {
		return template.JS(`{"labels":[],"visitors":[],"pageviews":[]}`) //nolint:gosec // Static literal.
	}

	encoded, err := json.Marshal(c)
	if err != nil {
		return template.JS(`{"labels":[],"visitors":[],"pageviews":[]}`) //nolint:gosec // Static literal.
	}

	//nolint:gosec // Payload is built from JSON-encoded, HTML-escaped numbers and labels.
	return template.JS(encoded)
}

// TableRow is a generic row used by the pages, referrers, devices, countries and
// events tables. Clicking a row toggles the matching dashboard filter.
type TableRow struct {
	// Label is the human readable value shown in the table.
	Label string
	// Secondary is an optional muted second line, e.g. a custom event value.
	Secondary string
	// FilterKey is the dashboard filter this row toggles, e.g. "path".
	FilterKey string
	// FilterValue is the raw value passed to the filter.
	FilterValue string
	// Icon is an optional Font Awesome class.
	Icon string

	Visitors     int
	VisitorsPct  float32
	Pageviews    int
	PageviewsPct float32
	Bounce       float32
	Duration     int
}

// TableView is the view model shared by every table partial.
type TableView struct {
	Hostname string
	// Title is the panel heading.
	Title string
	// Icon is the Font Awesome class for the panel heading.
	Icon string
	// Empty is the message shown when there are no rows.
	Empty string
	// LabelHeader is the heading of the first column.
	LabelHeader string
	// MetricHeader is the heading of the visitors column, e.g. "Events".
	MetricHeader string
	// Rows are the table rows.
	Rows []TableRow
	// Tabs is the list of tab definitions for tabbed panels.
	Tabs []Tab
	// TabKind names the Alpine state the tabs drive: "utm" or "tab".
	TabKind string
	// HasDetails shows the pageviews, bounce rate and duration columns.
	HasDetails bool
	// Pagination holds optional pagination controls and status.
	Pagination *Pagination
}

// Pagination holds pagination state for paginated table views.
type Pagination struct {
	Page       int
	PageSize   int
	TotalRows  int
	TotalPages int
	StartRow   int
	EndRow     int
	HasPrev    bool
	HasNext    bool
	PrevPage   int
	NextPage   int
	PageSizes  []int
}

// Tab is a single entry in a tabbed panel header.
type Tab struct {
	// Label is the tab caption.
	Label string
	// Icon is the Font Awesome class.
	Icon string
	// Value is the query parameter value, e.g. "browsers".
	Value string
	// Active marks the selected tab.
	Active bool
}

// HomeView is the view model for the website overview page.
type HomeView struct {
	Base
	Websites []WebsiteItem
}

// WebsiteItem is a single tracked website on the home page.
type WebsiteItem struct {
	Hostname string
	Visitors int
	// Snippet is the tracking snippet shown in the modal.
	Snippet string
}

// DashboardView is the view model for the analytics dashboard page.
//
// Every panel is rendered server-side on first paint so the dashboard is
// complete before any JavaScript runs; HTMX then replaces individual panels
// whenever the filters change.
type DashboardView struct {
	Base
	Filters   FilterState
	Periods   []Tab
	Summary   SummaryView
	Pages     TableView
	Referrers TableView
	Devices   TableView
	Countries TableView
	Events    TableView
	// Snippet is the tracking snippet, shown from the dashboard header.
	Snippet string
}

// FiltersJSON serialises the initial filter state for the Alpine component.
//
// The payload is embedded in a <script type="application/json"> element rather
// than inlined into an Alpine attribute, so filter values containing quotes or
// angle brackets cannot break out of the surrounding JavaScript.
func (v DashboardView) FiltersJSON() template.JS {
	payload := map[string]any{
		"hostname": v.Hostname,
		"period":   v.Filters.Period,
		"start":    v.Filters.Start,
		"end":      v.Filters.End,
		"utm":      v.Filters.UTM,
		"tab":      v.Filters.Tab,
		"filters": map[string]string{
			"path":        v.Filters.Path,
			"referrer":    v.Filters.Referrer,
			"country":     v.Filters.Country,
			"device":      v.Filters.Device,
			"browser":     v.Filters.Browser,
			"os":          v.Filters.OS,
			"utmSource":   v.Filters.UTMSource,
			"utmMedium":   v.Filters.UTMMedium,
			"utmCampaign": v.Filters.UTMCampaign,
		},
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return template.JS(`{}`) //nolint:gosec // Static literal.
	}

	//nolint:gosec // JSON-encoded with HTML escaping enabled by encoding/json.
	return template.JS(encoded)
}

// SettingsView is the view model for the account settings page.
type SettingsView struct {
	Base
	Profile ProfileView
	Usage   UsageView
	System  SystemView
}

// ProfileView holds the account form values.
type ProfileView struct {
	Username    string
	DateCreated int64
	DateUpdated int64
}

// UsageView holds the host resource usage block.
type UsageView struct {
	CPUUsage   float32
	CPUCores   int
	CPUThreads int
	MemoryUsed int64
	MemoryMax  int64
	DiskUsed   int64
	DiskMax    int64
	// MemoryPct and DiskPct are 0..1 ratios for the progress bars.
	MemoryPct float32
	DiskPct   float32
}

// SystemView holds the tenant-wide tracker settings.
type SystemView struct {
	ScriptType        string
	BlockAbusiveIPs   bool
	BlockTorExitNodes bool
	BlockedIPs        string
}

// Snippet returns the tracking snippet to paste into a tracked website.
//
// The tracker derives the hostname from `location.host` and the API endpoint
// from its own `src` attribute, so a single async script tag is all that is
// required. scriptURL must be absolute because it is loaded from another origin.
func Snippet(scriptURL string) string {
	return `<script async src="` + scriptURL + `"></script>`
}

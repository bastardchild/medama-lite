package ui

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/medama-io/medama/api"
	"github.com/medama-io/medama/db"
	"github.com/medama-io/medama/model"
	"github.com/medama-io/medama/util/logger"
)

const (
	// UnknownLabel is shown for events with an unrecognised value.
	UnknownLabel = "Unknown"
	// labelDirect is shown when a referrer or UTM dimension value is empty.
	labelDirect = "Direct / None"
	// metricVisitors is the heading of the visitor count column.
	metricVisitors = "Visitors"
)

// partialContext carries everything a panel builder needs after the hostname has
// been authorised and the filters have been resolved.
type partialContext struct {
	Hostname string
	State    FilterState
	Filters  *db.Filters
	Interval api.GetWebsiteIDSummaryInterval
}

// preparePartial authorises the hostname and builds the filter set. It writes an
// error response and returns false when the request should not continue.
func (s *Server) preparePartial(w http.ResponseWriter, r *http.Request) (*partialContext, bool) {
	hostname := r.PathValue("hostname")

	if _, ok := s.authorizeWebsite(r, hostname); !ok {
		s.renderError(w, r, http.StatusNotFound, "That website is not tracked on this account.")

		return nil, false
	}

	return s.partialContextFor(r, hostname), true
}

// partialContextFor builds the filter context for an already authorised host.
func (s *Server) partialContextFor(r *http.Request, hostname string) *partialContext {
	state := parseFilterState(r)
	start, end, interval := state.Resolve(timeNow())

	return &partialContext{
		Hostname: hostname,
		State:    state,
		Filters:  state.ToFilters(hostname, start, end),
		Interval: interval,
	}
}

// renderPartial renders a fragment, logging (but not masking) template errors.
func (s *Server) renderPartial(w http.ResponseWriter, name string, data any) {
	log := logger.Get()

	if err := s.renderer.Render(w, http.StatusOK, name, data); err != nil {
		log.Error().Err(err).Str("template", name).Msg("ui: failed to render partial")
	}
}

// handleSummaryPartial renders the summary cards and the timeseries payload.
func (s *Server) handleSummaryPartial(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()

	partial, ok := s.preparePartial(w, r)
	if !ok {
		return
	}

	view, err := s.buildSummary(r.Context(), partial)
	if err != nil {
		log.Error().Err(err).Msg("ui: failed to build summary")
		s.renderError(w, r, http.StatusInternalServerError, "Summary statistics are unavailable.")

		return
	}

	s.renderPartial(w, "partial_summary", view)
}

// handlePagesPartial renders the top pages table.
func (s *Server) handlePagesPartial(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()

	partial, ok := s.preparePartial(w, r)
	if !ok {
		return
	}

	view, err := s.buildPages(r.Context(), partial)
	if err != nil {
		log.Error().Err(err).Msg("ui: failed to build pages table")
		s.renderError(w, r, http.StatusInternalServerError, "Page statistics are unavailable.")

		return
	}

	s.renderPartial(w, "partial_table_pages", view)
}

// handleReferrersPartial renders top referrers, or the selected UTM dimension.
func (s *Server) handleReferrersPartial(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()

	partial, ok := s.preparePartial(w, r)
	if !ok {
		return
	}

	view, err := s.buildReferrers(r.Context(), partial)
	if err != nil {
		log.Error().Err(err).Msg("ui: failed to build referrers table")
		s.renderError(w, r, http.StatusInternalServerError, "Traffic sources are unavailable.")

		return
	}

	s.renderPartial(w, "partial_table_referrers", view)
}

// handleDevicesPartial renders browsers, operating systems or device types.
func (s *Server) handleDevicesPartial(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()

	partial, ok := s.preparePartial(w, r)
	if !ok {
		return
	}

	view, err := s.buildDevices(r.Context(), partial)
	if err != nil {
		log.Error().Err(err).Msg("ui: failed to build devices table")
		s.renderError(w, r, http.StatusInternalServerError, "Device data is unavailable.")

		return
	}

	s.renderPartial(w, "partial_table_devices", view)
}

// handleCountriesPartial renders the countries table.
func (s *Server) handleCountriesPartial(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()

	partial, ok := s.preparePartial(w, r)
	if !ok {
		return
	}

	view, err := s.buildCountries(r.Context(), partial)
	if err != nil {
		log.Error().Err(err).Msg("ui: failed to build countries table")
		s.renderError(w, r, http.StatusInternalServerError, "Country data is unavailable.")

		return
	}

	s.renderPartial(w, "partial_table_countries", view)
}

// handleEventsPartial renders custom events and their properties.
func (s *Server) handleEventsPartial(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()

	partial, ok := s.preparePartial(w, r)
	if !ok {
		return
	}

	view, err := s.buildEvents(r.Context(), partial)
	if err != nil {
		log.Error().Err(err).Msg("ui: failed to build events table")
		s.renderError(w, r, http.StatusInternalServerError, "Custom events are unavailable.")

		return
	}

	s.renderPartial(w, "partial_table_events", view)
}

// buildSummary loads the summary cards and the timeseries chart payload.
func (s *Server) buildSummary(ctx context.Context, partial *partialContext) (SummaryView, error) {
	summary, err := s.analytics.GetWebsiteSummary(ctx, partial.Filters)
	if err != nil {
		return SummaryView{}, fmt.Errorf("summary: %w", err)
	}

	metrics := summaryMetrics(summary, s.previousSummary(ctx, partial))

	intervals, err := s.analytics.GetWebsiteIntervals(ctx, partial.Filters, partial.Interval)
	if err != nil {
		return SummaryView{}, fmt.Errorf("intervals: %w", err)
	}

	chart := ChartData{
		Labels:    make([]string, 0, len(intervals)),
		Visitors:  make([]int, 0, len(intervals)),
		Pageviews: make([]int, 0, len(intervals)),
	}

	for _, interval := range intervals {
		chart.Labels = append(chart.Labels, formatInterval(interval.Interval))
		chart.Visitors = append(chart.Visitors, interval.Visitors)
		chart.Pageviews = append(chart.Pageviews, interval.Pageviews)
	}

	return SummaryView{Metrics: metrics, Chart: chart}, nil
}

// previousSummary loads the summary for the window immediately preceding the
// active period. Failures are non-fatal: the cards simply omit their deltas.
func (s *Server) previousSummary(ctx context.Context, partial *partialContext) *model.StatsSummarySingle {
	previousStart, previousEnd := PreviousPeriod(
		mustParse(partial.Filters.PeriodStart),
		mustParse(partial.Filters.PeriodEnd),
	)

	previousFilters := partial.State.ToFilters(partial.Hostname, previousStart, previousEnd)

	previous, err := s.analytics.GetWebsiteSummary(ctx, previousFilters)
	if err != nil {
		return nil
	}

	return previous
}

// summaryMetrics builds the four summary cards, including period-over-period
// deltas when a previous period is available.
func summaryMetrics(current, previous *model.StatsSummarySingle) []Metric {
	metrics := []Metric{
		{
			Label: "Unique visitors",
			Value: formatNumber(current.Visitors),
			Icon:  "fa-solid fa-users",
		},
		{
			Label: "Pageviews",
			Value: formatNumber(current.Pageviews),
			Icon:  "fa-solid fa-eye",
		},
		{
			Label: "Bounce rate",
			Value: formatPercent(current.BounceRate),
			Icon:  "fa-solid fa-arrow-trend-down",
		},
		{
			Label: "Visit duration",
			Value: formatDuration(current.Duration),
			Icon:  "fa-solid fa-clock",
		},
	}

	if previous == nil {
		return metrics
	}

	// Higher is better for visitors and pageviews, lower is better for the
	// bounce rate.
	applyIntDelta(&metrics[0], current.Visitors, previous.Visitors, true)
	applyIntDelta(&metrics[1], current.Pageviews, previous.Pageviews, true)
	applyFloatDelta(&metrics[2], current.BounceRate, previous.BounceRate, false)
	applyIntDelta(&metrics[3], current.Duration, previous.Duration, true)

	return metrics
}

// applyIntDelta fills in the delta fields of a card.
func applyIntDelta(metric *Metric, current, previous int, higherIsBetter bool) {
	if previous <= 0 || current <= 0 {
		return
	}

	change := (float64(current) - float64(previous)) / float64(previous) * percentScale
	up := change >= 0

	metric.Delta = fmt.Sprintf("%+.1f%%", change)
	metric.HasDelta = true
	metric.DeltaUp = up
	metric.Good = up == higherIsBetter
}

// applyFloatDelta fills in the delta fields of a card from a 0..1 ratio.
func applyFloatDelta(metric *Metric, current, previous float32, higherIsBetter bool) {
	if previous < 0 || current < 0 {
		return
	}

	change := (float64(current) - float64(previous)) / float64(previous) * percentScale
	up := change >= 0

	metric.Delta = fmt.Sprintf("%+.1f%%", change)
	metric.HasDelta = true
	metric.DeltaUp = up
	metric.Good = up == higherIsBetter
}

// buildPages loads the top pages table with server-side pagination.
func (s *Server) buildPages(ctx context.Context, partial *partialContext) (TableView, error) {
	pageSize := partial.State.PagesLimit
	if pageSize <= 0 {
		pageSize = 10
	}
	page := partial.State.PagesPage
	if page <= 0 {
		page = 1
	}

	pageFilters := *partial.Filters
	pageFilters.Limit = pageSize
	pageFilters.Offset = (page - 1) * pageSize

	pages, err := s.analytics.GetWebsitePages(ctx, &pageFilters)
	if err != nil {
		return TableView{}, fmt.Errorf("pages: %w", err)
	}

	totalRows, err := s.analytics.GetWebsitePagesCount(ctx, partial.Filters)
	if err != nil {
		totalRows = len(pages)
	}

	totalPages := 1
	if totalRows > 0 {
		totalPages = (totalRows + pageSize - 1) / pageSize
	}

	startRow := 0
	endRow := 0
	if totalRows > 0 {
		startRow = (page-1)*pageSize + 1
		endRow = startRow + len(pages) - 1
		if endRow > totalRows {
			endRow = totalRows
		}
	}

	rows := make([]TableRow, 0, len(pages))

	for _, p := range pages {
		rows = append(rows, TableRow{
			Label:        p.Pathname,
			FilterKey:    "path",
			FilterValue:  p.Pathname,
			Icon:         "fa-solid fa-file-lines",
			Visitors:     p.Visitors,
			VisitorsPct:  p.VisitorsPercentage,
			Pageviews:    p.Pageviews,
			PageviewsPct: p.PageviewsPercentage,
			Bounce:       p.BounceRate,
			Duration:     p.Duration,
		})
	}

	view := TableView{
		Hostname:     partial.Hostname,
		Title:        "Top pages",
		Icon:         "fa-solid fa-file-lines",
		Empty:        "No pageviews recorded for this period.",
		Rows:         rows,
		LabelHeader:  "Page",
		MetricHeader: metricVisitors,
		HasDetails:   true,
		Pagination: &Pagination{
			Page:       page,
			PageSize:   pageSize,
			TotalRows:  totalRows,
			TotalPages: totalPages,
			StartRow:   startRow,
			EndRow:     endRow,
			HasPrev:    page > 1,
			HasNext:    page < totalPages,
			PrevPage:   page - 1,
			NextPage:   page + 1,
			PageSizes:  []int{10, 25, 50},
		},
	}

	return view, nil
}

// buildReferrers loads the referrer panel for the selected UTM dimension.
func (s *Server) buildReferrers(ctx context.Context, partial *partialContext) (TableView, error) {
	view := TableView{
		Hostname: partial.Hostname,
		Icon:     "fa-solid fa-arrow-up-right-from-square",
		Empty:    "No traffic sources recorded for this period.",
		Tabs: []Tab{
			{Label: "Referrers", Value: UTMReferrer, Active: partial.State.UTM == UTMReferrer},
			{Label: "UTM source", Value: UTMSource, Active: partial.State.UTM == UTMSource},
			{Label: "UTM medium", Value: UTMMedium, Active: partial.State.UTM == UTMMedium},
			{Label: "UTM campaign", Value: UTMCampaign, Active: partial.State.UTM == UTMCampaign},
		},
	}
	view.MetricHeader = metricVisitors
	view.HasDetails = true
	view.TabKind = "utm"

	switch partial.State.UTM {
	case UTMSource:
		view.Title = "UTM sources"
		view.LabelHeader = "Source"

		sources, err := s.analytics.GetWebsiteUTMSources(ctx, partial.Filters)
		if err != nil {
			return TableView{}, fmt.Errorf("utm sources: %w", err)
		}

		for _, source := range sources {
			view.Rows = append(view.Rows, TableRow{
				Label:       defaultLabel(source.Source, labelDirect),
				FilterKey:   "utmSource",
				FilterValue: source.Source,
				Icon:        "fa-solid fa-tag",
				Visitors:    source.Visitors,
				VisitorsPct: source.VisitorsPercentage,
				Pageviews:   source.Pageviews,
				Bounce:      source.BounceRate,
				Duration:    source.Duration,
			})
		}

	case UTMMedium:
		view.Title = "UTM mediums"
		view.LabelHeader = "Medium"

		mediums, err := s.analytics.GetWebsiteUTMMediums(ctx, partial.Filters)
		if err != nil {
			return TableView{}, fmt.Errorf("utm mediums: %w", err)
		}

		for _, medium := range mediums {
			view.Rows = append(view.Rows, TableRow{
				Label:       defaultLabel(medium.Medium, labelDirect),
				FilterKey:   "utmMedium",
				FilterValue: medium.Medium,
				Icon:        "fa-solid fa-tag",
				Visitors:    medium.Visitors,
				VisitorsPct: medium.VisitorsPercentage,
				Pageviews:   medium.Pageviews,
				Bounce:      medium.BounceRate,
				Duration:    medium.Duration,
			})
		}

	case UTMCampaign:
		view.Title = "UTM campaigns"
		view.LabelHeader = "Campaign"

		campaigns, err := s.analytics.GetWebsiteUTMCampaigns(ctx, partial.Filters)
		if err != nil {
			return TableView{}, fmt.Errorf("utm campaigns: %w", err)
		}

		for _, campaign := range campaigns {
			view.Rows = append(view.Rows, TableRow{
				Label:       defaultLabel(campaign.Campaign, labelDirect),
				FilterKey:   "utmCampaign",
				FilterValue: campaign.Campaign,
				Icon:        "fa-solid fa-bullhorn",
				Visitors:    campaign.Visitors,
				VisitorsPct: campaign.VisitorsPercentage,
				Pageviews:   campaign.Pageviews,
				Bounce:      campaign.BounceRate,
				Duration:    campaign.Duration,
			})
		}

	default:
		view.Title = "Top referrers"
		view.LabelHeader = "Referrer"

		// Grouping collapses subdomains and paths into a single referrer group,
		// which matches how the dashboard has always presented this panel.
		referrers, err := s.analytics.GetWebsiteReferrers(ctx, true, partial.Filters)
		if err != nil {
			return TableView{}, fmt.Errorf("referrers: %w", err)
		}

		for _, referrer := range referrers {
			view.Rows = append(view.Rows, TableRow{
				Label:       defaultLabel(referrer.Referrer, labelDirect),
				FilterKey:   "referrer",
				FilterValue: referrer.Referrer,
				Icon:        "fa-solid fa-globe",
				Visitors:    referrer.Visitors,
				VisitorsPct: referrer.VisitorsPercentage,
				Pageviews:   referrer.Pageviews,
				Bounce:      referrer.BounceRate,
				Duration:    referrer.Duration,
			})
		}
	}

	return view, nil
}

// buildDevices loads the device panel for the selected tab.
func (s *Server) buildDevices(ctx context.Context, partial *partialContext) (TableView, error) {
	view := TableView{
		Hostname: partial.Hostname,
		Icon:     "fa-solid fa-laptop",
		Empty:    "No device data recorded for this period.",
		Tabs: []Tab{
			{Label: "Browsers", Value: TabBrowsers, Active: partial.State.Tab == TabBrowsers},
			{Label: "Operating systems", Value: TabOS, Active: partial.State.Tab == TabOS},
			{Label: "Device types", Value: TabDevices, Active: partial.State.Tab == TabDevices},
		},
	}
	view.MetricHeader = metricVisitors
	view.HasDetails = true
	view.TabKind = "tab"

	switch partial.State.Tab {
	case TabOS:
		view.Title = "Operating systems"
		view.LabelHeader = "Operating system"

		operatingSystems, err := s.analytics.GetWebsiteOS(ctx, partial.Filters)
		if err != nil {
			return TableView{}, fmt.Errorf("os: %w", err)
		}

		for _, item := range operatingSystems {
			view.Rows = append(view.Rows, TableRow{
				Label:       defaultLabel(item.OS, UnknownLabel),
				FilterKey:   "os",
				FilterValue: item.OS,
				Icon:        "fa-solid fa-microchip",
				Visitors:    item.Visitors,
				VisitorsPct: item.VisitorsPercentage,
				Pageviews:   item.Pageviews,
				Bounce:      item.BounceRate,
				Duration:    item.Duration,
			})
		}

	case TabDevices:
		view.Title = "Device types"
		view.LabelHeader = "Device type"

		devices, err := s.analytics.GetWebsiteDevices(ctx, partial.Filters)
		if err != nil {
			return TableView{}, fmt.Errorf("devices: %w", err)
		}

		for _, device := range devices {
			view.Rows = append(view.Rows, TableRow{
				Label:       defaultLabel(device.Device, UnknownLabel),
				FilterKey:   "device",
				FilterValue: device.Device,
				Icon:        deviceIcon(device.Device),
				Visitors:    device.Visitors,
				VisitorsPct: device.VisitorsPercentage,
				Pageviews:   device.Pageviews,
				Bounce:      device.BounceRate,
				Duration:    device.Duration,
			})
		}

	default:
		view.Title = "Browsers"
		view.LabelHeader = "Browser"

		browsers, err := s.analytics.GetWebsiteBrowsers(ctx, partial.Filters)
		if err != nil {
			return TableView{}, fmt.Errorf("browsers: %w", err)
		}

		for _, browser := range browsers {
			view.Rows = append(view.Rows, TableRow{
				Label:       defaultLabel(browser.Browser, UnknownLabel),
				FilterKey:   "browser",
				FilterValue: browser.Browser,
				Icon:        "fa-solid fa-window-maximize",
				Visitors:    browser.Visitors,
				VisitorsPct: browser.VisitorsPercentage,
				Pageviews:   browser.Pageviews,
				Bounce:      browser.BounceRate,
				Duration:    browser.Duration,
			})
		}
	}

	return view, nil
}

// buildCountries loads the countries table.
func (s *Server) buildCountries(ctx context.Context, partial *partialContext) (TableView, error) {
	countries, err := s.analytics.GetWebsiteCountries(ctx, partial.Filters)
	if err != nil {
		return TableView{}, fmt.Errorf("countries: %w", err)
	}

	rows := make([]TableRow, 0, len(countries))

	for _, country := range countries {
		rows = append(rows, TableRow{
			Label:       defaultLabel(country.Country, UnknownLabel),
			FilterKey:   "country",
			FilterValue: country.Country,
			Icon:        "fa-solid fa-location-dot",
			Visitors:    country.Visitors,
			VisitorsPct: country.VisitorsPercentage,
			Pageviews:   country.Pageviews,
			Bounce:      country.BounceRate,
			Duration:    country.Duration,
		})
	}

	view := TableView{
		Hostname: partial.Hostname,
		Title:    "Countries",
		Icon:     "fa-solid fa-earth-americas",
		Empty:    "No location data recorded for this period.",
		Rows:     rows,
	}
	view.LabelHeader = "Country"
	view.MetricHeader = metricVisitors
	view.HasDetails = true

	return view, nil
}

// buildEvents loads custom events and their properties.
func (s *Server) buildEvents(ctx context.Context, partial *partialContext) (TableView, error) {
	// Custom event queries read from the events table, so the period filters must
	// be applied to the event timestamps instead of the pageview timestamps.
	// A copy is used so the shared filter set stays untouched for other panels.
	eventFilters := *partial.Filters
	eventFilters.SortByEventDates = true

	properties, err := s.analytics.GetWebsiteCustomProperties(ctx, &eventFilters)
	if err != nil {
		return TableView{}, fmt.Errorf("custom events: %w", err)
	}

	rows := make([]TableRow, 0, len(properties))

	for _, property := range properties {
		rows = append(rows, TableRow{
			Label:       defaultLabel(property.Name, "Unnamed event"),
			Secondary:   property.Value,
			Icon:        "fa-solid fa-bolt",
			Visitors:    property.Events,
			VisitorsPct: property.EventsPercentage,
		})
	}

	view := TableView{
		Hostname: partial.Hostname,
		Title:    "Custom events",
		Icon:     "fa-solid fa-bolt",
		Empty:    "No custom events recorded for this period.",
		Rows:     rows,
	}
	view.LabelHeader = "Event"
	view.MetricHeader = "Events"

	return view, nil
}

// defaultLabel falls back to a placeholder when the analytics value is empty.
func defaultLabel(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

// deviceIcon picks an icon for a device type.
func deviceIcon(device string) string {
	switch device {
	case "mobile":
		return "fa-solid fa-mobile-screen"
	case "tablet":
		return "fa-solid fa-tablet-screen-button"
	case "desktop":
		return "fa-solid fa-desktop"
	default:
		return "fa-solid fa-circle-question"
	}
}

// mustParse parses an RFC3339 timestamp, returning the zero time on failure.
func mustParse(value string) time.Time {
	parsed, err := time.Parse(model.DateFormat, value)
	if err != nil {
		return time.Time{}
	}

	return parsed
}

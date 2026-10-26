package ui

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/medama-io/medama/api"
	"github.com/medama-io/medama/db"
	"github.com/medama-io/medama/model"
)

const (
	// PeriodToday covers the current day, bucketed hourly.
	PeriodToday = "today"
	// Period7d covers the trailing 7 days, bucketed daily.
	Period7d = "7d"
	// Period30d covers the trailing 30 days, bucketed daily.
	Period30d = "30d"
	// PeriodCustom uses explicit start/end dates.
	PeriodCustom = "custom"

	// DefaultTableLimit caps the number of rows returned for table panels.
	DefaultTableLimit = 100

	// UTM panel modes for the referrer panel.
	UTMReferrer = "referrer"
	UTMSource   = "source"
	UTMMedium   = "medium"
	UTMCampaign = "campaign"

	// Device panel modes.
	TabBrowsers = "browsers"
	TabOS       = "os"
	TabDevices  = "devices"

	// dateLayout is the format accepted for custom period boundaries.
	dateLayout = "2006-01-02"

	// periodDays7 and periodDays30 are the preset window lengths, in days.
	periodDays7  = 7
	periodDays30 = 30
	// hourlyRangeLimit is the longest custom range still bucketed hourly.
	hourlyRangeLimit = 48 * time.Hour
	// dayEndOffset extends a custom end date to the final second of that day.
	dayEndOffset = 24*time.Hour - time.Second
)

// FilterState mirrors the dashboard query parameters. It is shared by the page
// handler (to seed Alpine state) and by every HTMX partial handler.
type FilterState struct {
	Period   string
	Start    string
	End      string
	Path     string
	Referrer string
	Country  string
	Device   string
	Browser  string
	OS       string
	// UTMSource, UTMMedium and UTMCampaign are set by clicking a row in the
	// matching referrer panel tab.
	UTMSource   string
	UTMMedium   string
	UTMCampaign string
	// UTM selects the referrer panel mode: referrer, source, medium or campaign.
	UTM string
	// Tab selects the device panel mode: browsers, os or devices.
	Tab string
	// PagesPage selects the page number for top pages pagination (1-indexed).
	PagesPage int
	// PagesLimit selects the page size for top pages (10, 25, 50).
	PagesLimit int
}

// parseFilterState reads the dashboard filters from the request query string.
func parseFilterState(r *http.Request) FilterState {
	query := r.URL.Query()

	pagesPage := 1
	if p, err := strconv.Atoi(query.Get("page")); err == nil && p > 1 {
		pagesPage = p
	}

	pagesLimit := 10
	if l, err := strconv.Atoi(query.Get("pages_limit")); err == nil {
		if l == 25 || l == 50 {
			pagesLimit = l
		}
	}

	state := FilterState{
		Period:      query.Get("period"),
		Start:       query.Get("start"),
		End:         query.Get("end"),
		Path:        query.Get("path"),
		Referrer:    query.Get("referrer"),
		Country:     query.Get("country"),
		Device:      query.Get("device"),
		Browser:     query.Get("browser"),
		OS:          query.Get("os"),
		UTMSource:   query.Get("utm_source"),
		UTMMedium:   query.Get("utm_medium"),
		UTMCampaign: query.Get("utm_campaign"),
		UTM:         query.Get("utm"),
		Tab:         query.Get("tab"),
		PagesPage:   pagesPage,
		PagesLimit:  pagesLimit,
	}

	if state.Period == "" {
		state.Period = Period7d
	}

	if state.UTM == "" {
		state.UTM = UTMReferrer
	}

	if state.Tab == "" {
		state.Tab = TabBrowsers
	}

	return state
}

// ActiveFilters reports whether any row filter is currently applied.
func (s FilterState) ActiveFilters() bool {
	return s.Path != "" || s.Referrer != "" || s.Country != "" ||
		s.Device != "" || s.Browser != "" || s.OS != "" ||
		s.UTMSource != "" || s.UTMMedium != "" || s.UTMCampaign != ""
}

// HasCustomRange reports whether the period uses explicit dates.
func (s FilterState) HasCustomRange() bool {
	return s.Period == PeriodCustom
}

// Resolve converts the period selection into a concrete time window and the
// bucket interval used for the timeseries chart.
func (s FilterState) Resolve(now time.Time) (time.Time, time.Time, api.GetWebsiteIDSummaryInterval) {
	now = now.UTC()

	switch s.Period {
	case PeriodToday:
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		return start, now, api.GetWebsiteIDSummaryIntervalHour

	case Period30d:
		return now.AddDate(0, 0, -periodDays30), now, api.GetWebsiteIDSummaryIntervalDay

	case PeriodCustom:
		start, startErr := time.Parse(dateLayout, s.Start)
		end, endErr := time.Parse(dateLayout, s.End)

		if startErr == nil && endErr == nil && !end.Before(start) {
			// Make the end boundary inclusive of the whole selected day.
			end = end.Add(dayEndOffset)

			// Short ranges read better with hourly buckets.
			if end.Sub(start) <= hourlyRangeLimit {
				return start.UTC(), end.UTC(), api.GetWebsiteIDSummaryIntervalHour
			}

			return start.UTC(), end.UTC(), api.GetWebsiteIDSummaryIntervalDay
		}

		// Fall back to the default window when the custom range is unusable.
		return now.AddDate(0, 0, -periodDays7), now, api.GetWebsiteIDSummaryIntervalDay

	case Period7d:
		fallthrough
	default:
		return now.AddDate(0, 0, -periodDays7), now, api.GetWebsiteIDSummaryIntervalDay
	}
}

// ToFilters builds the DuckDB filter set for the analytics queries.
func (s FilterState) ToFilters(hostname string, start, end time.Time) *db.Filters {
	filters := &db.Filters{
		Hostname:    hostname,
		PeriodStart: start.Format(model.DateFormat),
		PeriodEnd:   end.Format(model.DateFormat),
		Limit:       DefaultTableLimit,
	}

	filters.Pathname = equalsFilter(db.FilterPathname, s.Path)
	filters.Referrer = equalsFilter(db.FilterReferrer, s.Referrer)
	filters.Country = equalsFilter(db.FilterCountry, s.Country)
	filters.Device = equalsFilter(db.FilterDevice, s.Device)
	filters.Browser = equalsFilter(db.FilterBrowser, s.Browser)
	filters.OS = equalsFilter(db.FilterOS, s.OS)
	filters.UTMSource = equalsFilter(db.FilterUTMSource, s.UTMSource)
	filters.UTMMedium = equalsFilter(db.FilterUTMMedium, s.UTMMedium)
	filters.UTMCampaign = equalsFilter(db.FilterUTMCampaign, s.UTMCampaign)

	return filters
}

// equalsFilter builds a case-insensitive equality filter, or nil when the value
// is empty so that the clause is omitted from the query entirely.
func equalsFilter(field db.FilterField, value string) *db.Filter {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	return &db.Filter{
		Field:     field,
		Value:     strings.ToLower(value),
		Operation: db.FilterEquals,
	}
}

// PreviousPeriod returns the window immediately preceding start..end, used to
// render the "vs previous period" deltas on the summary cards.
func PreviousPeriod(start, end time.Time) (time.Time, time.Time) {
	span := end.Sub(start)

	return start.Add(-span), start
}

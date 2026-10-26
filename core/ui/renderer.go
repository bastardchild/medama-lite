package ui

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Formatting constants shared by the template helpers.
const (
	// percentScale converts a 0..1 ratio into a percentage.
	percentScale = 100
	// percentDecimals is the number of decimal places used for percentages.
	percentDecimals = 1
	// thousandsGroup is the number of digits between thousands separators.
	thousandsGroup = 3
	// secondsPerMinute is used to split a duration into minutes and seconds.
	secondsPerMinute = 60
	// datePrefixLen is the length of the "2006-01-02" prefix of a timestamp.
	datePrefixLen = 10
	// float32Bits and float64Bits are the bit sizes passed to strconv.
	float32Bits = 32
	float64Bits = 64
	// decimalBase is the base used when rendering integers.
	decimalBase = 10
	// bytesPerUnit is the multiplier between consecutive binary byte units.
	bytesPerUnit = 1024
)

// Renderer parses and executes the embedded Go templates.
type Renderer struct {
	tpl *template.Template
}

// layoutView is implemented by every view model that is rendered inside the
// "base" layout. Embedding Base satisfies it, because Layout is promoted from
// the embedded *Base.
type layoutView interface {
	// Layout exposes the shared layout fields so RenderPage can attach the
	// rendered page markup before the layout is executed.
	Layout() *Base
}

// NewRenderer parses every embedded layout, page and partial into a single
// template namespace.
//
// The layout is executed by name ("base") and receives the already-rendered
// page markup in Base.Content. This avoids the duplicate-definition problem
// that arises when several pages each want to declare the same "content" block.
func NewRenderer() (*Renderer, error) {
	tpl, err := template.New("medama").
		Funcs(funcMap()).
		ParseFS(templatesFS,
			"templates/layouts/*.html",
			"templates/pages/*.html",
			"templates/partials/*.html",
		)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ui templates: %w", err)
	}

	return &Renderer{tpl: tpl}, nil
}

// Render executes the named template into a buffer and only writes the response
// once execution succeeded, so a template error can never produce a truncated
// body with a success status.
//
// It is used for HTMX fragments and for error fragments.
func (r *Renderer) Render(w http.ResponseWriter, status int, name string, data any) error {
	var buf bytes.Buffer

	if err := r.tpl.ExecuteTemplate(&buf, name, data); err != nil {
		return fmt.Errorf("failed to render template %q: %w", name, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if _, err := buf.WriteTo(w); err != nil {
		return fmt.Errorf("failed to write template %q: %w", name, err)
	}

	return nil
}

// RenderPage renders a page template and wraps it in the "base" layout.
//
// Go templates require the name in a {{template}} action to be a string
// constant, so the layout cannot select its content template dynamically.
// RenderPage therefore executes the page template first and injects the
// resulting markup into the layout through Base.Content.
func (r *Renderer) RenderPage(w http.ResponseWriter, status int, page string, view layoutView) error {
	var content bytes.Buffer

	if err := r.tpl.ExecuteTemplate(&content, page, view); err != nil {
		return fmt.Errorf("failed to render page template %q: %w", page, err)
	}

	//nolint:gosec // The markup is produced by our own embedded templates.
	view.Layout().Content = template.HTML(content.String())

	var buf bytes.Buffer

	if err := r.tpl.ExecuteTemplate(&buf, "base", view); err != nil {
		return fmt.Errorf("failed to render layout for page %q: %w", page, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if _, err := buf.WriteTo(w); err != nil {
		return fmt.Errorf("failed to write page %q: %w", page, err)
	}

	return nil
}

// Static returns the embedded static asset filesystem.
func Static() (fs.FS, error) {
	return fs.Sub(staticFS, "static")
}

// funcMap returns the helper functions available to every template. Only the
// helpers actually referenced by a template are registered, so a typo in a
// template fails loudly at startup instead of silently rendering nothing.
func funcMap() template.FuncMap {
	return template.FuncMap{
		"formatNumber":   formatNumber,
		"formatPercent":  formatPercent,
		"formatDuration": formatDuration,
		"formatDate":     formatDate,
		"formatBytes":    formatBytes,
		"formatFloat":    formatFloat,
		"barWidth":       barWidth,
		"contains":       strings.Contains,
	}
}

// formatNumber renders an integer with thousands separators, e.g. 1234567 ->
// "1,234,567".
func formatNumber(value int) string {
	negative := value < 0
	if negative {
		value = -value
	}

	digits := strconv.Itoa(value)

	var out strings.Builder
	if negative {
		out.WriteByte('-')
	}

	for i, digit := range digits {
		if i > 0 && (len(digits)-i)%thousandsGroup == 0 {
			out.WriteByte(',')
		}

		out.WriteRune(digit)
	}

	return out.String()
}

// formatPercent converts a 0..1 ratio into a human readable percentage, e.g.
// 0.4213 -> "42.1%". If ratio is negative, it indicates no data is available.
func formatPercent(ratio float32) string {
	if ratio < 0 {
		return "-"
	}

	return strconv.FormatFloat(float64(ratio)*percentScale, 'f', percentDecimals, float32Bits) + "%"
}

// formatDuration converts a duration in milliseconds into a compact label, e.g.
// 83000 -> "1m 23s". If ms <= 0, it indicates no duration data is available.
func formatDuration(ms int) string {
	if ms <= 0 {
		return "-"
	}

	duration := time.Duration(ms) * time.Millisecond

	switch {
	case duration < time.Minute:
		return strconv.Itoa(int(duration.Seconds())) + "s"
	case duration < time.Hour:
		minutes := int(duration.Minutes())
		seconds := int(duration.Seconds()) % secondsPerMinute

		if seconds == 0 {
			return strconv.Itoa(minutes) + "m"
		}

		return fmt.Sprintf("%dm %ds", minutes, seconds)
	default:
		hours := int(duration.Hours())
		minutes := int(duration.Minutes()) % 60

		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
}

// intervalLayouts are the timestamp formats DuckDB returns for bucketed
// intervals, in order of preference.
//
//nolint:gochecknoglobals // Immutable lookup table.
var intervalLayouts = []string{
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// formatInterval turns a bucketed interval timestamp into a short chart label.
func formatInterval(raw string) string {
	for _, layout := range intervalLayouts {
		parsed, err := time.Parse(layout, raw)
		if err != nil {
			continue
		}

		// Day-granularity buckets carry no time component worth showing.
		if parsed.Hour() == 0 && parsed.Minute() == 0 && !strings.Contains(raw, ":") {
			return parsed.Format("02 Jan")
		}

		return parsed.Format("02 Jan 15:04")
	}

	if len(raw) >= datePrefixLen {
		return raw[:datePrefixLen]
	}

	return raw
}

// formatDate renders a unix timestamp as a short date.
func formatDate(unix int64) string {
	if unix <= 0 {
		return "—"
	}

	return time.Unix(unix, 0).UTC().Format("02 Jan 2006")
}

// formatBytes renders a byte count using binary units, e.g. 1536 -> "1.5 KiB".
func formatBytes(value int64) string {
	if value <= 0 {
		return "0 B"
	}

	if value < bytesPerUnit {
		return strconv.FormatInt(value, decimalBase) + " B"
	}

	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}

	size := float64(value)

	var unit string

	for _, candidate := range units {
		size /= bytesPerUnit
		unit = candidate

		if size < bytesPerUnit {
			break
		}
	}

	return strconv.FormatFloat(size, 'f', percentDecimals, float64Bits) + " " + unit
}

// formatFloat renders a float with a single decimal place.
func formatFloat(value float32) string {
	return strconv.FormatFloat(float64(value), 'f', percentDecimals, float32Bits)
}

// barWidth converts a 0..1 ratio into a CSS width declaration for the
// percentage bars used in the tables.
func barWidth(ratio float32) template.CSS {
	if ratio < 0 {
		ratio = 0
	}

	if ratio > 1 {
		ratio = 1
	}

	//nolint:gosec // The value is clamped to a 0..100 float, so this cannot inject CSS.
	return template.CSS(
		"width:" + strconv.FormatFloat(
			float64(ratio)*percentScale, 'f', percentDecimals, float32Bits,
		) + "%",
	)
}

package ui

import (
	"net/http"

	"github.com/go-faster/errors"
	generate "github.com/medama-io/medama"
	"github.com/medama-io/medama/db/duckdb"
	"github.com/medama-io/medama/db/sqlite"
	"github.com/medama-io/medama/services"
	"github.com/medama-io/medama/util"
)

// Server renders the MedamaLite dashboard and exposes the HTMX endpoints that
// back it.
//
// Analytics reads go straight to the DuckDB client through the shared db.Filters
// abstraction, while anything that mutates application state (websites, users,
// tenant settings) is delegated to services.Handler so that the hostname cache
// used by the ingestion path stays consistent.
type Server struct {
	renderer        *Renderer
	auth            *util.AuthService
	svc             *services.Handler
	appDB           *sqlite.Client
	analytics       *duckdb.Client
	appDBPath       string
	analyticsDBPath string
	version         string
}

// NewServer builds the UI server.
func NewServer(
	auth *util.AuthService,
	svc *services.Handler,
	appDB *sqlite.Client,
	analytics *duckdb.Client,
	appDBPath string,
	analyticsDBPath string,
	version string,
) (*Server, error) {
	renderer, err := NewRenderer()
	if err != nil {
		return nil, err
	}

	return &Server{
		renderer:        renderer,
		auth:            auth,
		svc:             svc,
		appDB:           appDB,
		analytics:       analytics,
		appDBPath:       appDBPath,
		analyticsDBPath: analyticsDBPath,
		version:         version,
	}, nil
}

// Register wires every UI route onto the supplied mux.
func (s *Server) Register(mux *http.ServeMux) error {
	staticRoot, err := Static()
	if err != nil {
		return errors.Wrap(err, "failed to open embedded static assets")
	}

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticRoot)))
	mux.HandleFunc("GET /favicon.ico", s.handleFavicon)

	// Public routes.
	mux.HandleFunc("GET /login", s.handleLoginPage)
	mux.HandleFunc("POST /ui/auth/login", s.handleLoginSubmit)
	mux.HandleFunc("POST /ui/auth/logout", s.handleLogout)

	// Protected pages.
	mux.Handle("GET /{$}", s.requireSession(http.HandlerFunc(s.handleHome)))
	mux.Handle("GET /d/{hostname}", s.requireSession(http.HandlerFunc(s.handleDashboard)))
	mux.Handle("GET /settings", s.requireSession(http.HandlerFunc(s.handleSettingsPage)))
	mux.Handle("GET /openapi.yaml", s.requireSession(http.FileServer(http.FS(generate.OpenAPIDocument))))

	// Protected HTMX partials.
	mux.Handle("GET /ui/partials/{hostname}/summary", s.requireSession(http.HandlerFunc(s.handleSummaryPartial)))
	mux.Handle("GET /ui/partials/{hostname}/pages", s.requireSession(http.HandlerFunc(s.handlePagesPartial)))
	mux.Handle("GET /ui/partials/{hostname}/referrers", s.requireSession(http.HandlerFunc(s.handleReferrersPartial)))
	mux.Handle("GET /ui/partials/{hostname}/devices", s.requireSession(http.HandlerFunc(s.handleDevicesPartial)))
	mux.Handle("GET /ui/partials/{hostname}/countries", s.requireSession(http.HandlerFunc(s.handleCountriesPartial)))
	mux.Handle("GET /ui/partials/{hostname}/events", s.requireSession(http.HandlerFunc(s.handleEventsPartial)))

	// Website management.
	mux.Handle("GET /ui/websites", s.requireSession(http.HandlerFunc(s.handleWebsitesPartial)))
	mux.Handle("POST /ui/websites", s.requireSession(http.HandlerFunc(s.handleCreateWebsite)))
	mux.Handle("DELETE /ui/websites/{hostname}", s.requireSession(http.HandlerFunc(s.handleDeleteWebsite)))

	// Account management.
	mux.Handle("POST /ui/settings/profile", s.requireSession(http.HandlerFunc(s.handleUpdateProfile)))
	mux.Handle("POST /ui/settings/password", s.requireSession(http.HandlerFunc(s.handleUpdatePassword)))
	mux.Handle("POST /ui/settings/system", s.requireSession(http.HandlerFunc(s.handleUpdateSystem)))

	// Database backup & restore.
	mux.Handle("GET /ui/backup/download", s.requireSession(http.HandlerFunc(s.handleBackupDownload)))
	mux.Handle("POST /ui/backup/restore", s.requireSession(http.HandlerFunc(s.handleBackupRestore)))

	return nil
}

// handleFavicon serves the embedded favicon.
func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/static/favicon.svg", http.StatusMovedPermanently)
}

# Core

The MedamaLite server: Go API, DuckDB/SQLite persistence and the server-rendered
dashboard. Everything ships as a single binary.

## Development

- Use the Go version specified in the root `mise.toml`
- Ensure you have `gcc` and `g++` installed as CGO is required (`go-duckdb`)
- Install repository tools with `mise install` from the repository root
- Setup gofumpt and golangci-lint to automatically format and lint your code in IDE
- Run `go mod download`

From `core`, start the server with live reload:

```bash
mise run dev
```

The dashboard is served at <http://localhost:8080>. There is no separate frontend dev
server: `core/ui` renders pages and HTMX fragments with `html/template`, so editing
`ui/templates/*.html` or `ui/static/*` triggers an Air rebuild.

## Dashboard layout

```
ui/
├── embed.go                 //go:embed templates static
├── renderer.go              template parsing + helpers (formatNumber, formatDuration, ...)
├── filters.go               query params -> db.Filters, period resolution
├── middleware.go            session cookie middleware, HTMX helpers
├── handlers_auth.go         GET /login, POST /ui/auth/login, POST /ui/auth/logout
├── handlers_pages.go        GET /, GET /d/{hostname}, GET /settings
├── handlers_partials.go     panel builders + HTMX fragment handlers
├── handlers_settings.go     website CRUD, profile, password, tenant settings
├── handlers_backup.go       database export (live zip snapshot) & atomic restore
├── server.go                Server wiring and route registration
├── templates/               layouts, pages, partials
└── static/                  app.css, app.js, theme.js, favicon.svg
```

## Tracker scripts

`core/client/scripts/*.js` are frozen, pre-built artefacts embedded with `//go:embed`.
They are served from `/script.js` and `/scripts/*`. See `client/README.md` for how to
refresh them.

## Tests

- From `core`, run `mise run test`

## License

[Apache License 2.0](LICENSE)

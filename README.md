<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./.github/images/banner-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="./.github/images/banner-light.svg">
    <img alt="MedamaLite: Cookie-free privacy-focused website analytics." src="./.github/images/banner-light.svg">
  </picture>
</p>

## Overview

MedamaLite is a streamlined fork of [Medama Analytics](https://github.com/medama-io/medama) with the React/Remix SPA replaced by a **modern, high-density server-side rendered dashboard built directly into the Go binary**.

The entire application compiles into a single executable with no Node.js, Bun, or npm in either the build or runtime pipeline:

| Concern | Implementation |
| :--- | :--- |
| Pages & fragments | `html/template`, embedded via `//go:embed` in `core/ui` |
| Partial updates | [HTMX](https://htmx.org) |
| Client state (filters, modals, theme) | [Alpine.js](https://alpinejs.dev) |
| Styling & Design System | [Pico.css v2](https://picocss.com) + Custom High-Density Dark SaaS Design System |
| Typography & Geometry | Golden Ratio ($\phi = 1.618$) proportional scaling & button metrics |
| Icons | Font Awesome 6 |
| Charts | Chart.js 4 (Timeseries area & line charts) |
| Tracker script | Frozen, pre-built, embedded in `core/client/scripts` |

### Key Features

- 📊 **Real-Time Analytics** — Instant timeseries metrics with interactive click-to-filter breakdown panels (pages, referrers, UTM campaigns, devices, browsers, OS, countries, custom events).
- 🔒 **Privacy-First & Cookie-Free** — Ultra-lightweight sub-2 KB tracker script with no cookies, no persistent visitor fingerprinting, and no cross-site identifiers.
- 🎨 **Modern High-Density Dark UI** — Permanent obsidian dark palette (`#09090b` canvas, `#121215` card surface, `#10b981` emerald accent), floating segmented pill tabs, and flush edge-to-edge card architecture.
- 🛡️ **Hardened Security** — Built-in security headers (`X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`), with the OpenAPI schema (`/openapi.yaml`) protected behind authentication to prevent external reconnaissance.
- 💾 **Integrated Backup & Restore** — One-click online database backup (exporting both SQLite metadata and DuckDB analytics into a compressed ZIP) and atomic database restore directly from the Settings page.
- ⚡ **Ultra Lightweight** — Single multi-stage container build; idle memory consumption stays well under 50 MB RAM.

## Quick Start

Run MedamaLite using Docker Compose:

```bash
docker compose up -d --build
# Dashboard: http://127.0.0.1:8081
```

Both databases persist on the mounted `./data` host directory:

- `medama.db` — SQLite (user accounts, websites, tenant & traffic settings)
- `medama_analytics.duckdb` — DuckDB (raw analytics hits, views, and events)

## Local Development

```bash
cd core
mise run dev      # Hot reload with air
mise run test     # Run Go unit and integration tests
```

> [!NOTE]
> `gcc` and `g++` are required for local builds because `go-duckdb` utilizes CGO.

## Repository Layout

```
core/
├── api/         Generated ogen REST server handlers and schemas
├── client/      Frozen upstream tracker scripts (embedded via go:embed)
├── cmd/         CLI entrypoint and configuration commands (binary: medamalite)
├── db/          SQLite (application metadata) and DuckDB (analytics) clients
├── services/    Business logic, event ingestion pipeline, and hostname cache
└── ui/          Server-rendered templates, HTMX handlers, Alpine components, and static CSS
```

## Routes & Endpoints

| Route | Method | Access | Description |
| :--- | :--- | :--- | :--- |
| `/script.js`, `/scripts/*` | `GET` | Public | Frozen tracker scripts (ETag + 304 caching) |
| `/login` | `GET` | Public | Sign-in page |
| `/ui/auth/login` | `POST` | Public | Authenticate user session |
| `/ui/auth/logout` | `POST` | Authenticated | Terminate session cookie |
| `/` | `GET` | Authenticated | Websites overview dashboard |
| `/d/{hostname}` | `GET` | Authenticated | Detailed website analytics dashboard |
| `/settings` | `GET` | Authenticated | Account, privacy, tracker, and resource settings |
| `/openapi.yaml` | `GET` | Authenticated | Protected OpenAPI 3.1 REST API specification |
| `/ui/backup/download` | `GET` | Authenticated | Export live backup ZIP (SQLite + DuckDB + manifest) |
| `/ui/backup/restore` | `POST` | Authenticated | Restore database from uploaded backup ZIP |
| `/ui/websites` | `GET`, `POST` | Authenticated | Website list refresh and website creation |
| `/ui/websites/{hostname}` | `DELETE` | Authenticated | Delete website and purge associated analytics |
| `/ui/partials/{hostname}/{panel}` | `GET` | Authenticated | HTMX partial fragments (`summary`, `pages`, `referrers`, `devices`, `countries`, `events`) |
| `/api/*`, `/event/*` | Various | Mixed | Standard REST API and event tracking ingestion |

## License

- `/core` (including `core/ui` and `core/client`) is licensed under the Apache License 2.0 — see [core/LICENSE](./core/LICENSE).
- Tracker scripts in `core/client/scripts` originate from the upstream Medama tracker, licensed under the MIT License.

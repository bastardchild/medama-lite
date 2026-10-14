# Frozen tracker scripts

This directory contains the **pre-built** MedamaLite tracking script, embedded into the
Go binary with `//go:embed`. Nothing here is generated during a normal build, and no
Node.js / Bun runtime is required to build or run MedamaLite.

```
core/client/
├── client.go          # //go:embed all:scripts + accessors
└── scripts/           # frozen artefacts, served under /scripts/*
    ├── default.js
    ├── default.min.js
    ├── page-events.js
    ├── page-events.min.js
    ├── click-events.js
    ├── click-events.min.js
    ├── click-events.page-events.js
    └── click-events.page-events.min.js
```

## How the scripts are served

`services.TrackerHandler` (see `core/services/assets.go`) maps:

| Request | Served file |
| :--- | :--- |
| `GET /script.js` | the variant selected by the `script_type` tenant setting (defaults to `/scripts/default.js`) |
| `GET /scripts/{file}` | the matching frozen file |

Both routes send an `ETag` (CRC32 of the file contents) and honour `If-None-Match`,
returning `304 Not Modified` when unchanged. Script responses use
`public, max-age=21600, stale-while-revalidate=86400`.

The tracker variant is chosen by `services.convertScriptType`, which sorts the enabled
feature names alphabetically and joins them with `.`, e.g. `click-events,page-events` →
`/scripts/click-events.page-events.js`.

## Refreshing the tracker

The upstream `tracker/` Bun workspace was removed from this repository. To update the
tracking script:

1. Check out the upstream tracker in a separate working copy.
2. Run its build (`bun run build`) to populate `tracker/dist/`.
3. Copy `tracker/dist/*.js` into `core/client/scripts/`, overwriting the existing files.
4. Rebuild the server. No other change is required.

Verify integrity after copying, e.g.:

```sh
sha256sum core/client/scripts/*.js
```

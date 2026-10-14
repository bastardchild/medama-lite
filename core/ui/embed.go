// Package ui renders the MedamaLite dashboard server-side using html/template,
// HTMX for partial page updates and Alpine.js for client-side state.
//
// It replaces the React/Remix SPA that used to live in /dashboard. All page and
// fragment routes are registered on the main HTTP mux by Server.Register.
package ui

import "embed"

// Embedded Go templates: layouts, full pages and HTMX partials.
//
//go:embed templates
var templatesFS embed.FS

// Embedded static assets (stylesheet, Alpine components, chart renderer).
//
//go:embed static
var staticFS embed.FS

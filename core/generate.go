package generate

import (
	"embed"
)

// OpenAPI Codegen.
//
//go:generate bash ./generate.sh

// Embed OpenAPI Specification.
//
//go:embed openapi.yaml
var OpenAPIDocument embed.FS

// The React SPA client is no longer embedded here. MedamaLite renders all pages
// server-side from the `ui` package, and the frozen tracker scripts live in the
// `client` package.

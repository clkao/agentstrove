// ABOUTME: Embeds the compiled frontend dist directory into the Go binary.
// ABOUTME: Provides DistFS for serving static assets from the API server.
package web

import "embed"

//go:embed all:dist
var DistFS embed.FS

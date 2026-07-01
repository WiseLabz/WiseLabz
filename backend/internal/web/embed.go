// Package web embeds the built frontend SPA for production single-binary deployment.
package web

import "embed"

// DistFS holds the embedded SPA build output (web/dist/ copied here at build time).
// The all: prefix is required so Vite's dotfile manifests (e.g. .vite/manifest.json)
// aren't silently excluded by the default embed pattern.
//
//go:embed all:dist
var DistFS embed.FS

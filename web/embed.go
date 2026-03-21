// Package web holds the embedded frontend build output.
// Run `npm run build` in web/frontend/ to populate frontend/dist.
package web

import "embed"

// FS is the embedded frontend. It is intentionally unexported at package level;
// the internal/web/server.go package uses it via the exported StaticFS variable.
//
// The embed path is relative to this file (web/embed.go), so `frontend/dist`
// means `web/frontend/dist`.
//
//go:embed all:frontend/dist
var FS embed.FS

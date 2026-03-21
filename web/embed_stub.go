//go:build !web

package web

import "embed"

// FS is an empty filesystem used when the web UI is not embedded.
var FS embed.FS

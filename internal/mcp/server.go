package mcp

import (
	"github.com/mark3labs/mcp-go/server"
)

// NewServer creates and configures the MCP server with all tools registered.
func NewServer(d Deps) *server.MCPServer {
	srv := server.NewMCPServer(
		"alogin",
		"2.0",
		server.WithToolCapabilities(false),
	)
	RegisterTools(srv, d)
	return srv
}

// Serve starts the MCP server on stdio (blocking).
func Serve(d Deps) error {
	srv := NewServer(d)
	return server.ServeStdio(srv)
}

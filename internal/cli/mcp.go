package cli

import (
	"os"

	"github.com/emusal/alogin2/internal/mcp"
	"github.com/spf13/cobra"
)

func newMCPServerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp-server",
		Short: "Run alogin as an MCP (Model Context Protocol) server over stdio",
		Long: `Run alogin as an MCP (Model Context Protocol) server over stdio.

LLMs can use this server to query and manage alogin servers, tunnels,
and clusters. Communicate using JSON-RPC 2.0 messages over stdin/stdout.

Available tools:
  list_servers, get_server          — server registry queries
  list_tunnels, get_tunnel          — tunnel configuration queries
  start_tunnel, stop_tunnel         — tunnel lifecycle management
  list_clusters, get_cluster        — cluster queries with server details
  exec_command                      — run SSH commands on a single server
  exec_on_cluster                   — run SSH commands on all cluster servers in parallel

Example (Claude Desktop config):
  {
    "mcpServers": {
      "alogin": {
        "command": "alogin",
        "args": ["mcp-server"]
      }
    }
  }`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			binPath, _ := os.Executable()
			return mcp.Serve(mcp.Deps{
				DB:      database,
				Vault:   vlt,
				BinPath: binPath,
			})
		},
	}
}

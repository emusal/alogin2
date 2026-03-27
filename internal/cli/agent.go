package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/emusal/alogin2/internal/mcp"
	"github.com/spf13/cobra"
)

// newAgentCmd returns the "agent" group command for AI/MCP tooling.
func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "AI agent tools: MCP server, setup guide, and policy management",
		Long: `Tools for integrating alogin with AI agents (LLMs).

  alogin agent mcp     — run as an MCP server over stdio (for Claude Desktop, etc.)
  alogin agent setup   — print the MCP config and system prompt to copy into your AI client
  alogin agent policy  — manage HITL/RBAC safety policies (Phase 2)`,
	}
	cmd.AddCommand(
		newAgentMCPCmd(),
		newAgentSetupCmd(),
		newAgentPolicyCmd(),
	)
	return cmd
}

// newAgentMCPCmd runs the MCP server with audit logging enabled.
func newAgentMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "mcp",
		Short:        "Run alogin as an MCP server over stdio",
		SilenceUsage: true,
		Long: `Run alogin as an MCP (Model Context Protocol) server over stdio.

LLMs can use this server to query and manage alogin servers, tunnels,
and clusters. Communicates using JSON-RPC 2.0 over stdin/stdout.

All exec_command, exec_on_cluster, inspect_node, and log_analyzer calls are logged to the audit trail
at ~/.config/alogin/audit.jsonl (JSONL format).

Available tools:
  list_servers, get_server          — server registry queries
  list_tunnels, get_tunnel          — tunnel configuration queries
  start_tunnel, stop_tunnel         — tunnel lifecycle management
  list_clusters, get_cluster        — cluster queries with member details
  exec_command                      — run SSH commands on a single server
  exec_on_cluster                   — run SSH commands on all cluster servers in parallel
  inspect_node                      — structured health snapshot (CPU, mem, disk, processes)
  log_analyzer                      — stream logs and filter relevant error patterns`,
		RunE: func(cmd *cobra.Command, args []string) error {
			binPath, _ := os.Executable()

			var auditLog io.Writer
			if cfg != nil {
				auditPath := filepath.Join(filepath.Dir(cfg.DBPath), "audit.jsonl")
				if f, err := os.OpenFile(auditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600); err == nil {
					defer f.Close()
					auditLog = f
				}
			}

			return mcp.Serve(mcp.Deps{
				DB:       database,
				Vault:    vlt,
				BinPath:  binPath,
				AuditLog: auditLog,
			})
		},
	}
}

// newAgentSetupCmd prints the MCP configuration and system prompt for AI clients.
func newAgentSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Print MCP config and system prompt for AI clients (Claude Desktop, etc.)",
		Annotations: map[string]string{
			skipDBAnnotation: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			binPath, _ := os.Executable()

			auditPath := "~/.config/alogin/audit.jsonl"
			if cfg != nil {
				auditPath = filepath.Join(filepath.Dir(cfg.DBPath), "audit.jsonl")
			}

			fmt.Printf(`alogin — Security Gateway for Agentic AI
========================================

MCP server config (paste into Claude Desktop claude_desktop_config.json):

  {
    "mcpServers": {
      "alogin": {
        "command": %q,
        "args": ["agent", "mcp"]
      }
    }
  }

Recommended system prompt snippet:

  You have access to alogin, a secure SSH gateway for agentic infrastructure access.
  Use list_servers to discover available servers before connecting.
  Always provide an "intent" parameter when calling exec_command or exec_on_cluster
  to describe what you are doing and why.
  Do not run destructive commands (rm, shutdown, reboot) without explicit user confirmation.
  Prefer read-only inspection commands before modifying system state.

Available MCP tools (12):
  list_servers, get_server       — query server registry
  list_tunnels, get_tunnel       — tunnel configurations and status
  start_tunnel, stop_tunnel      — tunnel lifecycle
  list_clusters, get_cluster     — cluster groups with member details
  exec_command                   — run SSH commands on a single server
  exec_on_cluster                — run SSH commands on all cluster servers in parallel
  inspect_node                   — structured health snapshot (CPU, mem, disk, processes)
  log_analyzer                   — stream logs and filter relevant error patterns

Audit log: %s
  All exec_command, exec_on_cluster, inspect_node, and log_analyzer calls are logged here in JSONL format.
  Fields: timestamp, event, agent_id, server/cluster info, commands, intent.

LLM system prompt guide: docs/SYSTEM_PROMPT.md
  Copy the recommended snippet into your AI client's system prompt for safe agentic usage.

Ready-to-use config file: docs/mcp-config.json
  Copy-paste into claude_desktop_config.json (replace "alogin" with the full binary path if needed).
`, binPath, auditPath)
			return nil
		},
	}
}

// newAgentPolicyCmd is a Phase 2 stub for HITL/RBAC policy management.
func newAgentPolicyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "policy",
		Short: "HITL/RBAC policy management (Phase 2)",
		Annotations: map[string]string{
			skipDBAnnotation: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("agent policy: not yet implemented (Phase 2)")
			fmt.Println("")
			fmt.Println("Planned features:")
			fmt.Println("  - Command whitelists/blacklists per agent-id")
			fmt.Println("  - Time-of-day access restrictions")
			fmt.Println("  - Host group targeting policies")
			fmt.Println("  - Human-in-the-loop (HITL) approval prompts for destructive commands")
			return nil
		},
	}
}

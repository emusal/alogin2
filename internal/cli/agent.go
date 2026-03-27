package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/emusal/alogin2/internal/mcp"
	"github.com/emusal/alogin2/internal/policy"
	"github.com/spf13/cobra"
)

// newAgentCmd returns the "agent" group command for AI/MCP tooling.
func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "AI agent tools: MCP server, setup guide, and safety policy management",
		Long: `Tools for integrating alogin with AI agents (LLMs).

  alogin agent mcp            — run as an MCP server over stdio (for Claude Desktop, etc.)
  alogin agent setup          — print the MCP config and system prompt to copy into your AI client
  alogin agent policy         — manage global HITL/RBAC safety policies (show, validate)
  alogin agent audit          — query the structured MCP execution audit log
  alogin agent approve        — approve a pending HITL request
  alogin agent deny           — deny a pending HITL request
  alogin agent pending        — list pending HITL approval requests
  alogin agent server-policy  — manage per-server policy overrides (set/show/clear)
  alogin agent server-prompt  — manage per-server LLM system prompt overrides (set/show/clear)`,
	}
	cmd.AddCommand(
		newAgentMCPCmd(),
		newAgentSetupCmd(),
		newAgentPolicyCmd(),
		newAgentAuditCmd(),
		newAgentApproveCmd(),
		newAgentDenyCmd(),
		newAgentPendingCmd(),
		newAgentServerPolicyCmd(),
		newAgentServerPromptCmd(),
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

			var policyEngine *policy.Engine
			configDir := ""
			if cfg != nil {
				configDir = cfg.ConfigDir
				policyPath := filepath.Join(cfg.ConfigDir, "agent-policy.yaml")
				if pe, err := policy.LoadFile(policyPath); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to load agent policy: %v\n", err)
				} else {
					policyEngine = pe
				}
			}

			return mcp.Serve(mcp.Deps{
				DB:        database,
				Vault:     vlt,
				BinPath:   binPath,
				AuditLog:  auditLog,
				Policy:    policyEngine,
				ConfigDir: configDir,
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
  Query: alogin agent audit list [--agent <id>] [--server <id>] [--since 1h] [--json]

Safety policy (optional): ~/.config/alogin/agent-policy.yaml
  YAML file that controls what commands agents can run without human approval.
  Supports: command regex rules, agent-id globs, server/cluster scoping, time windows.
  Actions per rule: allow | deny | require_approval (HITL)
  Guide: docs/agent-policy.md   — full syntax reference with examples
  $ alogin agent policy show       — print active policy
  $ alogin agent policy validate   — check for syntax errors

Per-server overrides:
  $ alogin agent server-policy set <server-id> --file policy.yaml
  $ alogin agent server-prompt set <server-id> --text "Only run read-only commands on this host."

LLM system prompt guide: docs/SYSTEM_PROMPT.md
  Copy the recommended snippet into your AI client's system prompt for safe agentic usage.

Ready-to-use config file: docs/mcp-config.json
  Copy-paste into claude_desktop_config.json (replace "alogin" with the full binary path if needed).
`, binPath, auditPath)
			return nil
		},
	}
}

// newAgentPolicyCmd manages HITL/RBAC safety policies.
func newAgentPolicyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage HITL/RBAC safety policies",
		Long: `Manage the agent safety policy file (~/.config/alogin/agent-policy.yaml).

  alogin agent policy show      — print the active policy file
  alogin agent policy validate  — validate the policy file for syntax errors`,
		Annotations: map[string]string{skipDBAnnotation: "true"},
	}
	cmd.AddCommand(newAgentPolicyShowCmd(), newAgentPolicyValidateCmd())
	return cmd
}

func newAgentPolicyShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the active agent-policy.yaml",
		Annotations: map[string]string{skipDBAnnotation: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := policyFilePath()
			data, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				fmt.Printf("No policy file found at %s\n", path)
				fmt.Println("Built-in destructive-command patterns are active by default.")
				return nil
			}
			if err != nil {
				return err
			}
			fmt.Printf("# %s\n\n", path)
			fmt.Print(string(data))
			return nil
		},
	}
}

func newAgentPolicyValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate agent-policy.yaml for syntax and pattern errors",
		Annotations: map[string]string{skipDBAnnotation: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := policyFilePath()
			engine, err := policy.LoadFile(path)
			if os.IsNotExist(err) || (err == nil && engine == nil) {
				fmt.Printf("No policy file at %s — nothing to validate.\n", path)
				return nil
			}
			if err != nil {
				return fmt.Errorf("policy invalid: %w", err)
			}
			fmt.Printf("Policy is valid: %s\n", path)
			return nil
		},
	}
}

func policyFilePath() string {
	if cfg != nil {
		return filepath.Join(cfg.ConfigDir, "agent-policy.yaml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "alogin", "agent-policy.yaml")
}

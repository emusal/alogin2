package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/emusal/alogin2/internal/policy"
	"github.com/spf13/cobra"
)

// newAgentServerPolicyCmd manages per-server policy YAML overrides.
func newAgentServerPolicyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server-policy",
		Short: "Manage per-server policy overrides",
		Long: `Manage per-server agent policy overrides stored in the database.

When set, a server's policy completely replaces the global agent-policy.yaml
for commands targeting that server.

  alogin agent server-policy set   <server-id> [--file policy.yaml | --stdin]
  alogin agent server-policy show  <server-id>
  alogin agent server-policy clear <server-id>`,
	}
	cmd.AddCommand(
		newAgentServerPolicySetCmd(),
		newAgentServerPolicyShowCmd(),
		newAgentServerPolicyClearCmd(),
	)
	return cmd
}

func newAgentServerPolicySetCmd() *cobra.Command {
	var flagFile  string
	var flagStdin bool
	cmd := &cobra.Command{
		Use:   "set <server-id>",
		Short: "Set per-server policy YAML",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid server-id: %w", err)
			}

			var yamlData []byte
			switch {
			case flagStdin:
				yamlData, err = io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("read stdin: %w", err)
				}
			case flagFile != "":
				yamlData, err = os.ReadFile(flagFile)
				if err != nil {
					return fmt.Errorf("read file: %w", err)
				}
			default:
				return fmt.Errorf("provide --file <path> or --stdin")
			}

			yamlStr := string(yamlData)
			// Validate by parsing.
			if _, err := policy.ResolveFor(nil, yamlStr); err != nil {
				return fmt.Errorf("invalid policy YAML: %w", err)
			}

			srv, err := database.Servers.GetByID(ctx, id)
			if err != nil || srv == nil {
				return fmt.Errorf("server %d not found", id)
			}
			srv.PolicyYAML = yamlStr
			if err := database.Servers.Update(ctx, srv, ""); err != nil {
				return fmt.Errorf("update server: %w", err)
			}
			fmt.Printf("Server %d: policy set (%d bytes)\n", id, len(yamlStr))
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFile, "file", "", "Path to policy YAML file")
	cmd.Flags().BoolVar(&flagStdin, "stdin", false, "Read policy YAML from stdin")
	return cmd
}

func newAgentServerPolicyShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <server-id>",
		Short: "Show per-server policy YAML",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid server-id: %w", err)
			}
			srv, err := database.Servers.GetByID(ctx, id)
			if err != nil || srv == nil {
				return fmt.Errorf("server %d not found", id)
			}
			if srv.PolicyYAML == "" {
				fmt.Printf("Server %d (%s): no per-server policy (using global)\n", id, srv.Host)
				return nil
			}
			fmt.Printf("# Server %d (%s) policy:\n\n%s\n", id, srv.Host, srv.PolicyYAML)
			return nil
		},
	}
}

func newAgentServerPolicyClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear <server-id>",
		Short: "Remove per-server policy (revert to global)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid server-id: %w", err)
			}
			srv, err := database.Servers.GetByID(ctx, id)
			if err != nil || srv == nil {
				return fmt.Errorf("server %d not found", id)
			}
			srv.PolicyYAML = ""
			if err := database.Servers.Update(ctx, srv, ""); err != nil {
				return fmt.Errorf("update server: %w", err)
			}
			fmt.Printf("Server %d (%s): per-server policy cleared (now using global)\n", id, srv.Host)
			return nil
		},
	}
}

// newAgentServerPromptCmd manages per-server LLM system prompt overrides.
func newAgentServerPromptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server-prompt",
		Short: "Manage per-server LLM system prompt overrides",
		Long: `Manage per-server system prompt snippets stored in the database.

When set, the system_prompt field is included in get_server / list_servers
MCP responses so the LLM receives server-specific instructions.

  alogin agent server-prompt set   <server-id> --text "..." | --file prompt.txt | --stdin
  alogin agent server-prompt show  <server-id>
  alogin agent server-prompt clear <server-id>`,
	}
	cmd.AddCommand(
		newAgentServerPromptSetCmd(),
		newAgentServerPromptShowCmd(),
		newAgentServerPromptClearCmd(),
	)
	return cmd
}

func newAgentServerPromptSetCmd() *cobra.Command {
	var flagText  string
	var flagFile  string
	var flagStdin bool
	cmd := &cobra.Command{
		Use:   "set <server-id>",
		Short: "Set per-server system prompt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid server-id: %w", err)
			}

			var text string
			switch {
			case flagText != "":
				text = flagText
			case flagStdin:
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("read stdin: %w", err)
				}
				text = string(data)
			case flagFile != "":
				data, err := os.ReadFile(flagFile)
				if err != nil {
					return fmt.Errorf("read file: %w", err)
				}
				text = string(data)
			default:
				return fmt.Errorf("provide --text, --file <path>, or --stdin")
			}

			srv, err := database.Servers.GetByID(ctx, id)
			if err != nil || srv == nil {
				return fmt.Errorf("server %d not found", id)
			}
			srv.SystemPrompt = text
			if err := database.Servers.Update(ctx, srv, ""); err != nil {
				return fmt.Errorf("update server: %w", err)
			}
			fmt.Printf("Server %d (%s): system prompt set (%d bytes)\n", id, srv.Host, len(text))
			return nil
		},
	}
	cmd.Flags().StringVar(&flagText, "text", "", "Prompt text (inline)")
	cmd.Flags().StringVar(&flagFile, "file", "", "Path to prompt text file")
	cmd.Flags().BoolVar(&flagStdin, "stdin", false, "Read prompt from stdin")
	return cmd
}

func newAgentServerPromptShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <server-id>",
		Short: "Show per-server system prompt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid server-id: %w", err)
			}
			srv, err := database.Servers.GetByID(ctx, id)
			if err != nil || srv == nil {
				return fmt.Errorf("server %d not found", id)
			}
			if srv.SystemPrompt == "" {
				fmt.Printf("Server %d (%s): no per-server system prompt set\n", id, srv.Host)
				return nil
			}
			fmt.Printf("# Server %d (%s) system prompt:\n\n%s\n", id, srv.Host, srv.SystemPrompt)
			return nil
		},
	}
}

func newAgentServerPromptClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear <server-id>",
		Short: "Remove per-server system prompt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid server-id: %w", err)
			}
			srv, err := database.Servers.GetByID(ctx, id)
			if err != nil || srv == nil {
				return fmt.Errorf("server %d not found", id)
			}
			srv.SystemPrompt = ""
			if err := database.Servers.Update(ctx, srv, ""); err != nil {
				return fmt.Errorf("update server: %w", err)
			}
			fmt.Printf("Server %d (%s): system prompt cleared\n", id, srv.Host)
			return nil
		},
	}
}

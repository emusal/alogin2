package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/emusal/alogin2/internal/config"
	"github.com/emusal/alogin2/internal/policy"
	"github.com/spf13/cobra"
)

// trustScope builds the scope string from optional agent/server qualifiers.
// Without qualifiers it returns "global".
func trustScope(agentID string, serverID int64) string {
	if agentID != "" {
		return "agent:" + agentID
	}
	if serverID != 0 {
		return fmt.Sprintf("server:%d", serverID)
	}
	return "global"
}

// hitlConfigDir returns the config directory for HITL files.
// Uses the global cfg if available, otherwise loads it fresh.
func hitlConfigDir() (string, error) {
	if cfg != nil {
		return cfg.ConfigDir, nil
	}
	c, err := config.Load()
	if err != nil {
		return "", err
	}
	return c.ConfigDir, nil
}

func newAgentApproveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "approve <token>",
		Short: "Approve a pending HITL request",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{skipDBAnnotation: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			token := args[0]
			configDir, err := hitlConfigDir()
			if err != nil {
				return err
			}
			if err := policy.Approve(configDir, token); err != nil {
				return fmt.Errorf("approve: %w", err)
			}
			fmt.Printf("Approved: %s\n", token)
			return nil
		},
	}
}

func newAgentDenyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deny <token>",
		Short: "Deny a pending HITL request",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{skipDBAnnotation: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			token := args[0]
			configDir, err := hitlConfigDir()
			if err != nil {
				return err
			}
			if err := policy.Deny(configDir, token); err != nil {
				return fmt.Errorf("deny: %w", err)
			}
			fmt.Printf("Denied: %s\n", token)
			return nil
		},
	}
}

func newAgentPendingCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "pending",
		Short: "List pending HITL approval requests",
		Annotations: map[string]string{skipDBAnnotation: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			configDir, err := hitlConfigDir()
			if err != nil {
				return err
			}
			requests, err := policy.ListPending(configDir)
			if err != nil {
				return err
			}
			if len(requests) == 0 {
				fmt.Println("No pending approvals.")
				return nil
			}

			if flagJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(requests)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "TOKEN\tAGENT\tHOST\tCOMMANDS\tEXPIRES")
			for _, r := range requests {
				cmds := strings.Join(r.Commands, "; ")
				if len(cmds) > 50 {
					cmds = cmds[:47] + "..."
				}
				expires := r.ExpiresAt.Format(time.RFC3339)
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					r.Token,
					r.AgentID,
					r.Host,
					cmds,
					expires,
				)
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "Output as JSON")
	return cmd
}

func newAgentTrustCmd() *cobra.Command {
	var (
		duration string
		agentID  string
		serverID int64
	)
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "Grant a temporary trust window that auto-approves HITL requests",
		Long: `Grant a temporary trust window so that HITL (require_approval) requests are
automatically approved for the specified duration without waiting for a human.

Scope:
  (no flags)        global — applies to all agents and servers
  --agent <id>      only requests from this agent ID
  --server <id>     only requests targeting this server ID

Examples:
  alogin agent trust --duration 1h
  alogin agent trust --duration 30m --agent claude-dev
  alogin agent trust --duration 2h --server 3`,
		Annotations: map[string]string{skipDBAnnotation: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := time.ParseDuration(duration)
			if err != nil {
				return fmt.Errorf("invalid duration %q: %w", duration, err)
			}
			if d <= 0 {
				return fmt.Errorf("duration must be positive")
			}
			configDir, err := hitlConfigDir()
			if err != nil {
				return err
			}
			scope := trustScope(agentID, serverID)
			tw, err := policy.GrantTrust(configDir, scope, d, "")
			if err != nil {
				return fmt.Errorf("trust: %w", err)
			}
			fmt.Printf("Trust window granted\n  scope:   %s\n  expires: %s\n",
				tw.Scope, tw.ExpiresAt.Format(time.RFC3339))
			return nil
		},
	}
	cmd.Flags().StringVar(&duration, "duration", "1h", "How long to trust (e.g. 30m, 2h)")
	cmd.Flags().StringVar(&agentID, "agent", "", "Restrict to this agent ID (default: all agents)")
	cmd.Flags().Int64Var(&serverID, "server", 0, "Restrict to this server ID (default: all servers)")
	return cmd
}

func newAgentUntrustCmd() *cobra.Command {
	var (
		agentID  string
		serverID int64
	)
	cmd := &cobra.Command{
		Use:   "untrust",
		Short: "Revoke an active trust window",
		Annotations: map[string]string{skipDBAnnotation: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			configDir, err := hitlConfigDir()
			if err != nil {
				return err
			}
			scope := trustScope(agentID, serverID)
			if err := policy.RevokeTrust(configDir, scope); err != nil {
				return fmt.Errorf("untrust: %w", err)
			}
			fmt.Printf("Trust window revoked: %s\n", scope)
			return nil
		},
	}
	cmd.Flags().StringVar(&agentID, "agent", "", "Agent scope to revoke")
	cmd.Flags().Int64Var(&serverID, "server", 0, "Server scope to revoke")
	return cmd
}

func newAgentTrustListCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "trust-list",
		Short: "List active trust windows",
		Annotations: map[string]string{skipDBAnnotation: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			configDir, err := hitlConfigDir()
			if err != nil {
				return err
			}
			windows, err := policy.ListTrust(configDir)
			if err != nil {
				return err
			}
			if len(windows) == 0 {
				fmt.Println("No active trust windows.")
				return nil
			}
			if flagJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(windows)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SCOPE\tGRANTED AT\tEXPIRES\tREMAINING")
			for _, tw := range windows {
				remaining := time.Until(tw.ExpiresAt).Round(time.Second)
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					tw.Scope,
					tw.GrantedAt.Format(time.RFC3339),
					tw.ExpiresAt.Format(time.RFC3339),
					remaining,
				)
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "Output as JSON")
	return cmd
}

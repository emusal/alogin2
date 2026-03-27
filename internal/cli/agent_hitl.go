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

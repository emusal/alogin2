package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/emusal/alogin2/internal/db"
	"github.com/spf13/cobra"
)

func newAgentAuditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Query the MCP execution audit log",
		Long: `Query the structured audit log of all MCP exec events.

  alogin agent audit list   — list recent audit entries
  alogin agent audit tail   — stream new entries as they arrive`,
	}
	cmd.AddCommand(newAgentAuditListCmd(), newAgentAuditTailCmd())
	return cmd
}

func newAgentAuditListCmd() *cobra.Command {
	var (
		flagAgent  string
		flagServer int64
		flagEvent  string
		flagSince  string
		flagLimit  int
		flagFormat string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent audit log entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			opts := db.AuditListOpts{
				AgentID:   flagAgent,
				EventType: flagEvent,
				Limit:     flagLimit,
			}
			if flagServer != 0 {
				opts.ServerID = &flagServer
			}
			if flagSince != "" {
				t, err := parseSinceDuration(flagSince)
				if err != nil {
					return fmt.Errorf("--since: %w", err)
				}
				opts.Since = t
			}

			entries, err := database.AuditLog.List(ctx, opts)
			if err != nil {
				return err
			}

			if flagFormat == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}

			if len(entries) == 0 {
				fmt.Println("No audit entries found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "TIME\tEVENT\tAGENT\tHOST/CLUSTER\tCOMMANDS\tPOLICY\tINTENT")
			for _, e := range entries {
				target := e.ServerHost
				if e.ClusterName != "" {
					target = "[cluster] " + e.ClusterName
				}
				cmds := strings.Join(e.Commands, "; ")
				if len(cmds) > 60 {
					cmds = cmds[:57] + "..."
				}
				intent := e.Intent
				if len(intent) > 40 {
					intent = intent[:37] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					e.CreatedAt.Format("01-02 15:04:05"),
					e.Event,
					e.AgentID,
					target,
					cmds,
					e.PolicyAction,
					intent,
				)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&flagAgent, "agent", "", "Filter by agent_id")
	cmd.Flags().Int64Var(&flagServer, "server", 0, "Filter by server ID")
	cmd.Flags().StringVar(&flagEvent, "event", "", "Filter by event type (exec_command, exec_on_cluster, inspect_node, log_analyzer)")
	cmd.Flags().StringVar(&flagSince, "since", "", "Show entries since (e.g. 1h, 24h, or RFC3339 timestamp)")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum number of entries to return")
	cmd.Flags().StringVar(&flagFormat, "format", "table", "output format: table|json")
	return cmd
}

func newAgentAuditTailCmd() *cobra.Command {
	var flagFormat string
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Stream new audit log entries as they arrive",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()

			lastSeen := time.Now()
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

			fmt.Fprintln(os.Stderr, "Tailing audit log (Ctrl+C to stop)...")

			for {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(2 * time.Second):
				}

				opts := db.AuditListOpts{
					Since: lastSeen,
					Limit: 100,
				}
				entries, err := database.AuditLog.List(ctx, opts)
				if err != nil {
					return err
				}

				// List returns newest-first; reverse for tail display.
				for i := len(entries) - 1; i >= 0; i-- {
					e := entries[i]
					if e.CreatedAt.After(lastSeen) {
						lastSeen = e.CreatedAt
					}
					if flagFormat == "json" {
						enc := json.NewEncoder(os.Stdout)
						_ = enc.Encode(e)
					} else {
						target := e.ServerHost
						if e.ClusterName != "" {
							target = "[cluster] " + e.ClusterName
						}
						cmds := strings.Join(e.Commands, "; ")
						if len(cmds) > 60 {
							cmds = cmds[:57] + "..."
						}
						fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
							e.CreatedAt.Format("01-02 15:04:05"),
							e.Event,
							e.AgentID,
							target,
							cmds,
						)
						_ = w.Flush()
					}
				}
			}
		},
	}
	cmd.Flags().StringVar(&flagFormat, "format", "table", "output format: table|json")
	return cmd
}

// parseSinceDuration parses a --since value: a Go duration (e.g. "1h") or an RFC3339 timestamp.
func parseSinceDuration(s string) (time.Time, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d), nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("expected a duration (e.g. 1h) or RFC3339 timestamp")
	}
	return t, nil
}

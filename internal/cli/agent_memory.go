package cli

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// newAgentServerMemoryCmd manages per-server AI agent memory notes.
func newAgentServerMemoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server-memory",
		Short: "Manage per-server AI agent memory notes",
		Long: `Manage free-form natural language notes that AI agents save per server.

Notes are append-only and timestamped. Use them to record observations,
findings, or reminders about a server for future agent sessions.

  alogin agent server-memory add  <server-id> --text "..."
  alogin agent server-memory list <server-id>
  alogin agent server-memory del  <server-id> <note-id>`,
	}
	cmd.AddCommand(
		newAgentServerMemoryAddCmd(),
		newAgentServerMemoryListCmd(),
		newAgentServerMemoryDelCmd(),
	)
	return cmd
}

func newAgentServerMemoryAddCmd() *cobra.Command {
	var flagText string
	cmd := &cobra.Command{
		Use:   "add <server-id>",
		Short: "Add a memory note for a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid server-id: %w", err)
			}
			if flagText == "" {
				return fmt.Errorf("provide --text \"note content\"")
			}
			srv, err := database.Servers.GetByID(ctx, id)
			if err != nil || srv == nil {
				return fmt.Errorf("server %d not found", id)
			}
			entry, err := database.AgentMemory.Add(ctx, id, flagText)
			if err != nil {
				return fmt.Errorf("add memory: %w", err)
			}
			fmt.Printf("Server %d (%s): memory note #%d saved\n", id, srv.Host, entry.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagText, "text", "", "Note content")
	return cmd
}

func newAgentServerMemoryListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <server-id>",
		Short: "List memory notes for a server",
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
			entries, err := database.AgentMemory.ListByServer(ctx, id)
			if err != nil {
				return fmt.Errorf("list memory: %w", err)
			}
			if len(entries) == 0 {
				fmt.Printf("Server %d (%s): no memory notes\n", id, srv.Host)
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tCREATED\tNOTE")
			for _, e := range entries {
				fmt.Fprintf(w, "%d\t%s\t%s\n", e.ID, e.CreatedAt.Format("2006-01-02 15:04:05"), e.Content)
			}
			w.Flush()
			return nil
		},
	}
}

func newAgentServerMemoryDelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "del <server-id> <note-id>",
		Short: "Delete a memory note",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			serverID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid server-id: %w", err)
			}
			noteID, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid note-id: %w", err)
			}
			srv, err := database.Servers.GetByID(ctx, serverID)
			if err != nil || srv == nil {
				return fmt.Errorf("server %d not found", serverID)
			}
			if err := database.AgentMemory.Delete(ctx, noteID); err != nil {
				return fmt.Errorf("delete memory: %w", err)
			}
			fmt.Printf("Server %d (%s): memory note #%d deleted\n", serverID, srv.Host, noteID)
			return nil
		},
	}
}

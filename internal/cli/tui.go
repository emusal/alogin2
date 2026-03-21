package cli

import (
	"context"
	"fmt"

	"github.com/emusal/alogin2/internal/model"
	"github.com/emusal/alogin2/internal/tui"
	"github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// newTUICmd exposes the TUI as a standalone command — starts at the welcome screen.
func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Interactive host selector",
		Long:  `Launch the interactive TUI. Starts at the welcome screen.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUIAt(context.Background(), tui.StartAtWelcome)
		},
	}
}

// runConnectTUIFull launches the TUI directly at the server list (used by `alogin connect`).
func runConnectTUIFull(ctx context.Context, opts *model.ConnectOptions) error {
	return runTUIAtWithOpts(ctx, tui.StartAtList, opts)
}

// runTUIAt launches the TUI starting at the given section.
func runTUIAt(ctx context.Context, start tui.StartAt) error {
	return runTUIAtWithOpts(ctx, start, &model.ConnectOptions{})
}

// runTUIAtWithOpts is the core TUI launcher.
func runTUIAtWithOpts(ctx context.Context, start tui.StartAt, opts *model.ConnectOptions) error {
	servers, err := database.Servers.ListAll(ctx)
	if err != nil {
		return err
	}

	m := tui.NewModelAt(servers, database, start)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	result, ok := finalModel.(tui.Model)
	if !ok {
		return nil
	}
	choice := result.Choice()
	if choice == nil {
		return nil // user quit without selecting
	}

	return doConnect(ctx, choice.User, choice.Server.Host, opts)
}

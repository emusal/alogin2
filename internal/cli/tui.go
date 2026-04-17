package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbletea"
	"github.com/emusal/alogin2/internal/cluster"
	"github.com/emusal/alogin2/internal/model"
	"github.com/emusal/alogin2/internal/tui"
	"github.com/spf13/cobra"
)

// newTUICmd exposes the TUI as a standalone command — starts at the welcome screen.
func newTUICmd() *cobra.Command {
	var startSection string
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Interactive host selector",
		Long:  `Launch the interactive TUI. Starts at the welcome screen by default.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			start := tui.StartAtWelcome
			switch startSection {
			case "server":
				start = tui.StartAtList
			case "gateway":
				start = tui.StartAtGateway
			case "profile":
				start = tui.StartAtProfile
			case "cluster":
				start = tui.StartAtCluster
			case "hosts":
				start = tui.StartAtHosts
			case "tunnel":
				start = tui.StartAtTunnel
			case "app-server":
				start = tui.StartAtAppServer
			}
			return runTUIAt(context.Background(), start)
		},
	}
	cmd.Flags().StringVar(&startSection, "start", "", "section to open: server|gateway|profile|cluster|hosts|tunnel|app-server")
	return cmd
}

// runConnectTUIFull launches the TUI directly at the server list (used by `alogin access`).
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

	pluginDir := ""
	if cfg != nil {
		pluginDir = cfg.ConfigDir + "/plugins"
	}
	m := tui.NewModelAt(servers, database, start, Version, pluginDir)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	result, ok := finalModel.(tui.Model)
	if !ok {
		return nil
	}

	if cl := result.ChoiceCluster(); cl != nil {
		return runClusterFromTUI(ctx, cl.Cluster)
	}

	choice := result.Choice()
	if choice == nil {
		return nil // user quit without selecting
	}

	if choice.Plugin != "" {
		opts.AppName = choice.Plugin
	}
	return doConnect(ctx, choice.User, choice.Server.Host, opts)
}

// runClusterFromTUI opens cluster SSH sessions for the given cluster (called after TUI selection).
func runClusterFromTUI(ctx context.Context, cl *model.Cluster) error {
	var hosts []cluster.HostEntry
	for _, m := range cl.Members {
		srv, err := database.Servers.GetByID(ctx, m.ServerID)
		if err != nil || srv == nil {
			continue
		}
		user := srv.User
		if m.User != "" {
			user = m.User
		}
		pwd, _ := vlt.Get(vaultKey(srv))
		hosts = append(hosts, cluster.HostEntry{
			Host:     srv.Host,
			Port:     srv.EffectivePort(),
			User:     user,
			Password: pwd,
		})
	}
	if len(hosts) == 0 {
		return fmt.Errorf("no hosts in cluster %s", cl.Name)
	}
	binPath, _ := os.Executable()
	mgr := cluster.NewManager("tmux", 0, binPath)
	return mgr.Open(ctx, cl.Name, hosts)
}

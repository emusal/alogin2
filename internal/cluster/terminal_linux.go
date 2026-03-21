//go:build !darwin

package cluster

import (
	"context"
	"fmt"
)

// openTerminalApp is not available on Linux — redirect to tmux.
func openTerminalApp(ctx context.Context, name string, hosts []HostEntry, tileX int, binPath string) error {
	fmt.Println("Note: Terminal.app is macOS-only, falling back to tmux.")
	return openTmux(ctx, name, hosts, tileX, binPath)
}

// openITerm is not available on Linux — redirect to tmux.
func openITerm(ctx context.Context, name string, hosts []HostEntry, tileX int, binPath string) error {
	fmt.Println("Note: iTerm2 is macOS-only, falling back to tmux.")
	return openTmux(ctx, name, hosts, tileX, binPath)
}

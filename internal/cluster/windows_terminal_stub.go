//go:build !windows

package cluster

import (
	"context"
	"fmt"
)

// openWindowsTerminal is a stub on non-Windows platforms.
func openWindowsTerminal(ctx context.Context, name string, hosts []HostEntry, tileX int, binPath string) error {
	fmt.Println("Note: Windows Terminal is Windows-only, falling back to tmux.")
	return openTmux(ctx, name, hosts, tileX, binPath)
}

//go:build windows

package cluster

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// openWindowsTerminal opens a cluster session using Windows Terminal (wt.exe).
// Requires Windows Terminal (winget install Microsoft.WindowsTerminal) — Win 10 2004+.
func openWindowsTerminal(ctx context.Context, name string, hosts []HostEntry, tileX int, binPath string) error {
	if len(hosts) == 0 {
		return fmt.Errorf("no hosts")
	}

	// wt.exe new-tab --title <name> -- <cmd0> ; split-pane -- <cmd1> ; ...
	args := []string{"new-tab", "--title", "alogin-" + name, "--"}
	args = append(args, strings.Fields(buildConnCmd(binPath, hosts[0]))...)

	for _, h := range hosts[1:] {
		args = append(args, ";", "split-pane", "--")
		args = append(args, strings.Fields(buildConnCmd(binPath, h))...)
	}

	return exec.CommandContext(ctx, "wt.exe", args...).Start()
}

// openTerminalApp and openITerm are macOS-only — fall back to Windows Terminal.
func openTerminalApp(ctx context.Context, name string, hosts []HostEntry, tileX int, binPath string) error {
	fmt.Println("Note: Terminal.app is macOS-only, using Windows Terminal.")
	return openWindowsTerminal(ctx, name, hosts, tileX, binPath)
}

func openITerm(ctx context.Context, name string, hosts []HostEntry, tileX int, binPath string) error {
	fmt.Println("Note: iTerm2 is macOS-only, using Windows Terminal.")
	return openWindowsTerminal(ctx, name, hosts, tileX, binPath)
}

//go:build darwin

package cluster

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// openTerminalApp opens each host in a separate Terminal.app window using AppleScript.
func openTerminalApp(ctx context.Context, name string, hosts []HostEntry, tileX int, binPath string) error {
	script := buildTerminalScript(hosts, binPath)
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Terminal.app AppleScript: %w\n%s", err, out)
	}
	return nil
}

func buildTerminalScript(hosts []HostEntry, binPath string) string {
	var sb strings.Builder
	sb.WriteString("tell application \"Terminal\"\n")
	sb.WriteString("  activate\n")
	for _, h := range hosts {
		connCmd := buildConnCmd(binPath, h)
		sb.WriteString(fmt.Sprintf("  do script \"%s\"\n", escapeAppleScript(connCmd)))
	}
	sb.WriteString("end tell\n")
	return sb.String()
}

func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

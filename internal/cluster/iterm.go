//go:build darwin

package cluster

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// openITerm opens all hosts in iTerm2 using split panes.
func openITerm(ctx context.Context, name string, hosts []HostEntry, tileX int, binPath string) error {
	script := buildITermScript(hosts, tileX, binPath)
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("iTerm2 AppleScript: %w\n%s", err, out)
	}
	return nil
}

func buildITermScript(hosts []HostEntry, tileX int, binPath string) string {
	if tileX <= 0 {
		tileX = 2
	}
	var sb strings.Builder
	sb.WriteString(`tell application "iTerm2"` + "\n")
	sb.WriteString("  activate\n")
	sb.WriteString("  set newWindow to (create window with default profile)\n")
	sb.WriteString("  tell newWindow\n")
	sb.WriteString("    tell current session\n")

	for i, h := range hosts {
		connCmd := buildConnCmd(binPath, h)
		if i == 0 {
			sb.WriteString(fmt.Sprintf("      write text \"%s\"\n", escapeAppleScript(connCmd)))
		} else {
			if i%tileX == 0 {
				sb.WriteString("      set newRow to (split horizontally with default profile)\n")
				sb.WriteString("      tell newRow\n")
				sb.WriteString(fmt.Sprintf("        write text \"%s\"\n", escapeAppleScript(connCmd)))
				sb.WriteString("      end tell\n")
			} else {
				sb.WriteString("      set newPane to (split vertically with default profile)\n")
				sb.WriteString("      tell newPane\n")
				sb.WriteString(fmt.Sprintf("        write text \"%s\"\n", escapeAppleScript(connCmd)))
				sb.WriteString("      end tell\n")
			}
		}
	}

	sb.WriteString("    end tell\n")
	sb.WriteString("  end tell\n")
	sb.WriteString("end tell\n")
	return sb.String()
}

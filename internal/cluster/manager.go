// Package cluster manages simultaneous SSH sessions for a cluster of hosts.
// On macOS, Terminal.app and iTerm2 are supported. On all platforms, tmux is used.
package cluster

import "context"

// HopEntry is one gateway hop for a cluster member.
type HopEntry struct {
	Host     string
	Port     int
	User     string
	Password string
}

// HostEntry describes one member of a cluster session.
type HostEntry struct {
	Host     string
	Port     int
	User     string
	Password string
	Hops     []HopEntry // gateway chain (may be empty for direct)
	UseGW    bool       // whether to force auto-gw flag for child processes
}

// Manager opens cluster sessions using the configured mode.
type Manager struct {
	mode    string
	tileX   int
	binPath string // path to alogin binary; if set, panes use self-invocation
}

// NewManager creates a Manager with the given mode and tile columns.
// binPath is the path to the alogin binary (os.Executable()); pass "" to fall
// back to plain ssh commands.
func NewManager(mode string, tileX int, binPath string) *Manager {
	if mode == "" {
		mode = "tmux"
	}
	return &Manager{mode: mode, tileX: tileX, binPath: binPath}
}

// Open launches SSH sessions for all hosts in the cluster.
func (m *Manager) Open(ctx context.Context, clusterName string, hosts []HostEntry) error {
	switch m.mode {
	case "iterm":
		return openITerm(ctx, clusterName, hosts, m.tileX, m.binPath)
	case "terminal":
		return openTerminalApp(ctx, clusterName, hosts, m.tileX, m.binPath)
	default:
		return openTmux(ctx, clusterName, hosts, m.tileX, m.binPath)
	}
}

// buildConnCmd returns the command string to run in a pane/window for one host.
//
// When binPath is set, it uses "alogin connect [--auto-gw] user@host" so that
// the alogin process handles vault lookup and SSH password injection
// programmatically — no terminal password prompt, which eliminates the
// synchronize-panes cross-contamination bug.
//
// When binPath is empty (fallback), it builds a plain ssh command.
func buildConnCmd(binPath string, h HostEntry) string {
	if binPath != "" {
		cmd := binPath + " access ssh"
		if h.UseGW || len(h.Hops) > 0 {
			cmd += " --auto-gw"
		}
		cmd += " " + h.User + "@" + h.Host
		return cmd
	}
	return buildSSHCmd(h)
}

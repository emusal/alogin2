package cli

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// ── newAgentSetupCmd ──────────────────────────────────────────────────────────

// newAgentSetupCmd prints the MCP configuration and system prompt for AI clients.
func newAgentSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Print MCP config and system prompt for AI clients (Claude Desktop, etc.)",
		Annotations: map[string]string{
			skipDBAnnotation: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			binPath, _ := os.Executable()

			auditPath := "~/.config/alogin/audit.jsonl"
			if cfg != nil {
				auditPath = filepath.Join(filepath.Dir(cfg.DBPath), "audit.jsonl")
			}

			fmt.Printf(`alogin — Security Gateway for Agentic AI
========================================

MCP server config (paste into Claude Desktop claude_desktop_config.json):

  {
    "mcpServers": {
      "alogin": {
        "command": %q,
        "args": ["agent", "mcp"]
      }
    }
  }

Cursor (cursor.mcpServers in settings.json):

  "cursor.mcpServers": {
    "alogin": {
      "command": %q,
      "args": ["agent", "mcp"]
    }
  }

VS Code (mcp.servers in settings.json):

  "mcp": {
    "servers": {
      "alogin": {
        "type": "stdio",
        "command": %q,
        "args": ["agent", "mcp"]
      }
    }
  }

Recommended system prompt snippet:

  You have access to alogin, a secure SSH gateway for agentic infrastructure access.
  Use list_servers to discover available servers before connecting.
  Always provide an "intent" parameter when calling exec_command or exec_on_cluster
  to describe what you are doing and why.
  Do not run destructive commands (rm, shutdown, reboot) without explicit user confirmation.
  Prefer read-only inspection commands before modifying system state.

Available MCP tools (12):
  list_servers, get_server       — query server registry
  list_tunnels, get_tunnel       — tunnel configurations and status
  start_tunnel, stop_tunnel      — tunnel lifecycle
  list_clusters, get_cluster     — cluster groups with member details
  exec_command                   — run SSH commands on a single server
  exec_on_cluster                — run SSH commands on all cluster servers in parallel
  inspect_node                   — structured health snapshot (CPU, mem, disk, processes)
  log_analyzer                   — stream logs and filter relevant error patterns

Audit log: %s
  All exec_command, exec_on_cluster, inspect_node, and log_analyzer calls are logged here in JSONL format.
  Fields: timestamp, event, agent_id, server/cluster info, commands, intent.
  Query: alogin agent audit list [--agent <id>] [--server <id>] [--since 1h] [--json]

Safety policy (optional): ~/.config/alogin/agent-policy.yaml
  YAML file that controls what commands agents can run without human approval.
  Supports: command regex rules, agent-id globs, server/cluster scoping, time windows.
  Actions per rule: allow | deny | require_approval (HITL)
  Guide: docs/agent-policy.md   — full syntax reference with examples
  $ alogin agent policy show       — print active policy
  $ alogin agent policy validate   — check for syntax errors

Per-server overrides:
  $ alogin agent server-policy set <server-id> --file policy.yaml
  $ alogin agent server-prompt set <server-id> --text "Only run read-only commands on this host."

LLM system prompt guide: docs/SYSTEM_PROMPT.md
  Copy the recommended snippet into your AI client's system prompt for safe agentic usage.

Ready-to-use config file: docs/mcp-config.json
  Copy-paste into claude_desktop_config.json (replace "alogin" with the full binary path if needed).
`, binPath, binPath, binPath, auditPath)
			return nil
		},
	}
}

// ── newAgentSkillsCmd ─────────────────────────────────────────────────────────

func newAgentSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage alogin agent skills",
		Long: `Manage alogin skills installed to ~/.agents/skills.

  alogin agent skills install           — download and install skills from GitHub
  alogin agent skills list              — list installed skills`,
		Annotations: map[string]string{skipDBAnnotation: "true"},
	}
	cmd.AddCommand(newAgentSkillsInstallCmd(), newAgentSkillsListCmd())
	return cmd
}

func newAgentSkillsInstallCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "install [skill...]",
		Short: "Download and install alogin skills from GitHub",
		Long: `Download skills.tar.gz from the latest alogin GitHub release and install
skills to the target directory.

Without skill arguments, lists available skills and prompts you to choose.
With skill arguments, installs only the named skills directly.
Pass "all" to install everything without prompting.

Skills are installed as:
  <dir>/<skill-name>/SKILL.md

Examples:
  alogin agent skills install                      # interactive selection
  alogin agent skills install all                  # install all
  alogin agent skills install alogin alogin-cli    # install specific skills
  alogin agent skills install --dir ~/my-skills    # custom destination`,
		Annotations: map[string]string{skipDBAnnotation: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				dir = filepath.Join(homeDir(), ".agents", "skills")
			}
			// Fetch archive into memory first so we can show the skill list.
			skills, err := fetchSkillsArchive()
			if err != nil {
				return err
			}

			var selected []skillEntry
			switch {
			case len(args) == 1 && args[0] == "all":
				selected = skills
			case len(args) > 0:
				selected, err = filterSkills(skills, args)
				if err != nil {
					return err
				}
			default:
				selected, err = pickSkillsInteractive(skills)
				if err != nil {
					return err
				}
			}

			if len(selected) == 0 {
				fmt.Println("No skills selected.")
				return nil
			}
			return writeSkills(selected, dir)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Install destination (default: ~/.agents/skills)")
	return cmd
}

func newAgentSkillsListCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List installed alogin skills",
		Annotations: map[string]string{skipDBAnnotation: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				dir = filepath.Join(homeDir(), ".agents", "skills")
			}
			entries, err := os.ReadDir(dir)
			if os.IsNotExist(err) {
				fmt.Printf("No skills directory found at %s\n", dir)
				fmt.Println("Run:  alogin agent skills install")
				return nil
			}
			if err != nil {
				return err
			}
			fmt.Printf("Skills in %s:\n\n", dir)
			found := 0
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				skill := filepath.Join(dir, e.Name(), "SKILL.md")
				if !fileExists(skill) {
					continue
				}
				fmt.Printf("  %-20s  %s\n", e.Name(), skill)
				found++
			}
			if found == 0 {
				fmt.Println("  (none)")
				fmt.Println("\nRun:  alogin agent skills install")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Skills directory to list (default: ~/.agents/skills)")
	return cmd
}

// ── Skills fetch ──────────────────────────────────────────────────────────────

const skillsURL = "https://github.com/emusal/alogin2/releases/latest/download/skills.tar.gz"

// skillEntry holds the name and content of one skill fetched from the archive.
type skillEntry struct {
	Name    string
	Content []byte
}

// fetchSkillsArchive downloads skills.tar.gz and returns all skills in memory.
func fetchSkillsArchive() ([]skillEntry, error) {
	fmt.Printf("Fetching skills from %s ...\n", skillsURL)

	resp, err := http.Get(skillsURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("download skills: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download skills: HTTP %d", resp.StatusCode)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("decompress skills: %w", err)
	}
	defer gz.Close()

	var skills []skillEntry
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read skills archive: %w", err)
		}

		// Archive layout: ./<skill-name>/SKILL.md
		clean := filepath.Clean(hdr.Name)
		parts := strings.Split(clean, string(os.PathSeparator))
		if len(parts) != 2 || parts[1] != "SKILL.md" {
			continue
		}
		name := parts[0]
		if name == "" || name == "." {
			continue
		}

		data, err := io.ReadAll(io.LimitReader(tr, 4<<20))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		skills = append(skills, skillEntry{Name: name, Content: data})
	}

	if len(skills) == 0 {
		return nil, fmt.Errorf("no skills found in archive")
	}
	return skills, nil
}

// filterSkills returns only the skills whose names match the given list.
func filterSkills(skills []skillEntry, names []string) ([]skillEntry, error) {
	index := make(map[string]skillEntry, len(skills))
	for _, s := range skills {
		index[s.Name] = s
	}
	var out []skillEntry
	for _, name := range names {
		s, ok := index[name]
		if !ok {
			available := make([]string, 0, len(skills))
			for _, sk := range skills {
				available = append(available, sk.Name)
			}
			return nil, fmt.Errorf("unknown skill %q (available: %s)", name, strings.Join(available, ", "))
		}
		out = append(out, s)
	}
	return out, nil
}

// ── Checkbox TUI (bubbletea) ──────────────────────────────────────────────────

var (
	styleChecked   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)  // green
	styleUnchecked = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))              // gray
	styleCursor    = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)  // blue
	styleHint      = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true) // gray italic
)

type checkboxModel struct {
	items    []skillEntry
	checked  []bool
	cursor   int
	quitting bool
	aborted  bool
}

func newCheckboxModel(items []skillEntry) checkboxModel {
	return checkboxModel{
		items:   items,
		checked: make([]bool, len(items)),
	}
}

func (m checkboxModel) Init() tea.Cmd { return nil }

func (m checkboxModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.aborted = true
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case " ":
			m.checked[m.cursor] = !m.checked[m.cursor]
		case "a":
			// toggle all: if all checked → uncheck all, else check all
			allChecked := true
			for _, c := range m.checked {
				if !c {
					allChecked = false
					break
				}
			}
			for i := range m.checked {
				m.checked[i] = !allChecked
			}
		case "enter":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m checkboxModel) View() string {
	if m.quitting {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\nSelect skills to install:\n\n")
	for i, item := range m.items {
		cursor := "  "
		if i == m.cursor {
			cursor = styleCursor.Render("▶ ")
		}
		checkbox := styleUnchecked.Render("☐")
		name := item.Name
		if m.checked[i] {
			checkbox = styleChecked.Render("☑")
			name = styleChecked.Render(name)
		}
		sb.WriteString(fmt.Sprintf("%s%s  %s\n", cursor, checkbox, name))
	}
	sb.WriteString("\n")
	sb.WriteString(styleHint.Render("space: toggle  •  a: toggle all  •  enter: confirm  •  q: quit"))
	sb.WriteString("\n")
	return sb.String()
}

// pickSkillsInteractive shows a checkbox TUI and returns the selected skills.
func pickSkillsInteractive(skills []skillEntry) ([]skillEntry, error) {
	m := newCheckboxModel(skills)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("skill picker: %w", err)
	}
	final := result.(checkboxModel)
	if final.aborted {
		return nil, nil
	}
	var selected []skillEntry
	for i, c := range final.checked {
		if c {
			selected = append(selected, skills[i])
		}
	}
	return selected, nil
}

// writeSkills writes skill entries to destDir/<name>/SKILL.md.
func writeSkills(skills []skillEntry, destDir string) error {
	if err := os.MkdirAll(destDir, 0750); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}
	for _, s := range skills {
		skillDir := filepath.Join(destDir, s.Name)
		outPath := filepath.Join(skillDir, "SKILL.md")

		// Guard against path traversal
		if !strings.HasPrefix(filepath.Clean(outPath), filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}
		if err := os.MkdirAll(skillDir, 0750); err != nil {
			return err
		}
		if err := os.WriteFile(outPath, s.Content, 0640); err != nil {
			return err
		}
		fmt.Printf("  installed: %-20s → %s\n", s.Name, outPath)
	}
	fmt.Printf("Done. %d skill(s) installed to %s\n", len(skills), destDir)
	return nil
}

// installSkillsFromRelease installs all skills (used from interactive setup flow).
func installSkillsFromRelease(destDir string) error {
	skills, err := fetchSkillsArchive()
	if err != nil {
		return err
	}
	return writeSkills(skills, destDir)
}

// ── OS helpers ────────────────────────────────────────────────────────────────

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return os.Getenv("HOME")
}

func xdgConfig() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	return filepath.Join(homeDir(), ".config")
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// winCodePath returns VS Code / Cursor paths on Windows using the registry.
// Stub — actual registry lookup not needed since we use APPDATA.
func winCodePath() string { return "" }

// openEditor opens a file in the system default editor (best-effort, no error).
func openEditor(path string) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		return
	}
	_ = exec.Command(editor, path).Start()
}

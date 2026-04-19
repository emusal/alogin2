package cli

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// mcpEntry is the JSON shape written into each client's config file.
type mcpEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// clientDef describes a supported MCP client.
type clientDef struct {
	Name        string
	ConfigPath  func() string // returns expanded path; empty string = not found
	MergeConfig func(path, binPath string) error
}

var supportedClients = []clientDef{
	{
		Name:        "cursor",
		ConfigPath:  cursorConfigPath,
		MergeConfig: mergeCursorConfig,
	},
	{
		Name:        "claude-desktop",
		ConfigPath:  claudeDesktopConfigPath,
		MergeConfig: mergeClaudeDesktopConfig,
	},
	{
		Name:        "vscode",
		ConfigPath:  vscodeConfigPath,
		MergeConfig: mergeVSCodeConfig,
	},
}

// ── newAgentSetupCmd ──────────────────────────────────────────────────────────

func newAgentSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup [client]",
		Short: "Register alogin as an MCP server in AI clients",
		Long: `Register alogin as an MCP server in supported AI clients.

Supported clients: cursor, claude-desktop, vscode, all

Without a client argument, runs interactively and prompts you to choose.
"all" detects every installed client and registers alogin in each.

Examples:
  alogin agent setup                  # interactive: pick client(s)
  alogin agent setup cursor           # register in Cursor only
  alogin agent setup claude-desktop   # register in Claude Desktop
  alogin agent setup vscode           # register in VS Code
  alogin agent setup all              # all detected clients`,
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			skipDBAnnotation: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			binPath, _ := os.Executable()

			target := ""
			if len(args) > 0 {
				target = strings.ToLower(args[0])
			}

			switch target {
			case "":
				return setupInteractive(binPath)
			case "all":
				return setupAll(binPath)
			default:
				return setupClient(target, binPath)
			}
		},
	}
	return cmd
}

// setupInteractive prompts the user to choose clients and whether to install skills.
func setupInteractive(binPath string) error {
	fmt.Println("alogin — MCP setup")
	fmt.Println()

	// Detect installed clients
	detected := detectClients()
	if len(detected) == 0 {
		fmt.Println("No supported AI clients detected.")
		fmt.Println()
		fmt.Println("Supported clients: cursor, claude-desktop, vscode")
		fmt.Println("Run:  alogin agent setup <client>  to configure one manually.")
		return nil
	}

	fmt.Println("Detected clients:")
	for i, c := range detected {
		fmt.Printf("  [%d] %s\n", i+1, c.Name)
	}
	fmt.Println("  [a] all")
	fmt.Println("  [q] quit")
	fmt.Println()
	fmt.Print("Select client(s) [1]: ")

	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(input)
	if input == "" {
		input = "1"
	}

	var chosen []clientDef
	switch input {
	case "q", "Q":
		return nil
	case "a", "A", "all":
		chosen = detected
	default:
		var idx int
		if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(detected) {
			return fmt.Errorf("invalid selection %q", input)
		}
		chosen = []clientDef{detected[idx-1]}
	}

	for _, c := range chosen {
		if err := registerClient(c, binPath); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %s: %v\n", c.Name, err)
		}
	}

	fmt.Println()
	fmt.Print("Install alogin skills to ~/.agent/skills? [Y/n]: ")
	var skillInput string
	fmt.Scanln(&skillInput)
	skillInput = strings.TrimSpace(strings.ToLower(skillInput))
	if skillInput == "" || skillInput == "y" || skillInput == "yes" {
		defaultDir := filepath.Join(homeDir(), ".agent", "skills")
		return installSkillsFromRelease(defaultDir)
	}

	return nil
}

// setupAll registers in every detected client without prompting.
func setupAll(binPath string) error {
	detected := detectClients()
	if len(detected) == 0 {
		fmt.Println("No supported AI clients detected.")
		return nil
	}
	for _, c := range detected {
		if err := registerClient(c, binPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", c.Name, err)
		}
	}
	return nil
}

// setupClient registers a single named client.
func setupClient(name, binPath string) error {
	for _, c := range supportedClients {
		if c.Name == name {
			return registerClient(c, binPath)
		}
	}
	names := make([]string, len(supportedClients))
	for i, c := range supportedClients {
		names[i] = c.Name
	}
	return fmt.Errorf("unknown client %q (supported: %s)", name, strings.Join(names, ", "))
}

// registerClient merges the MCP entry into one client's config file.
func registerClient(c clientDef, binPath string) error {
	path := c.ConfigPath()
	if path == "" {
		fmt.Printf("  %-18s config not found (skipping)\n", c.Name)
		return nil
	}
	if err := c.MergeConfig(path, binPath); err != nil {
		return err
	}
	fmt.Printf("  %-18s registered → %s\n", c.Name, path)
	return nil
}

// detectClients returns clients whose config path can be resolved.
func detectClients() []clientDef {
	var out []clientDef
	for _, c := range supportedClients {
		if c.ConfigPath() != "" {
			out = append(out, c)
		}
	}
	return out
}

// ── Config path resolvers ─────────────────────────────────────────────────────

func cursorConfigPath() string {
	// Cursor stores settings in the same location as VS Code but under "Cursor"
	var base string
	switch runtime.GOOS {
	case "darwin":
		base = filepath.Join(homeDir(), "Library", "Application Support", "Cursor", "User")
	case "linux":
		base = filepath.Join(xdgConfig(), "Cursor", "User")
	case "windows":
		base = filepath.Join(os.Getenv("APPDATA"), "Cursor", "User")
	}
	if base == "" {
		return ""
	}
	p := filepath.Join(base, "settings.json")
	if fileExists(p) || dirExists(base) {
		return p
	}
	return ""
}

func claudeDesktopConfigPath() string {
	var p string
	switch runtime.GOOS {
	case "darwin":
		p = filepath.Join(homeDir(), "Library", "Application Support", "Claude", "claude_desktop_config.json")
	case "linux":
		p = filepath.Join(xdgConfig(), "Claude", "claude_desktop_config.json")
	case "windows":
		p = filepath.Join(os.Getenv("APPDATA"), "Claude", "claude_desktop_config.json")
	}
	if p == "" {
		return ""
	}
	if fileExists(p) || dirExists(filepath.Dir(p)) {
		return p
	}
	return ""
}

func vscodeConfigPath() string {
	var base string
	switch runtime.GOOS {
	case "darwin":
		base = filepath.Join(homeDir(), "Library", "Application Support", "Code", "User")
	case "linux":
		base = filepath.Join(xdgConfig(), "Code", "User")
	case "windows":
		base = filepath.Join(os.Getenv("APPDATA"), "Code", "User")
	}
	if base == "" {
		return ""
	}
	p := filepath.Join(base, "settings.json")
	if fileExists(p) || dirExists(base) {
		return p
	}
	return ""
}

// ── Config merge helpers ──────────────────────────────────────────────────────

// mergeCursorConfig writes `cursor.mcpServers.alogin` into Cursor's settings.json.
func mergeCursorConfig(path, binPath string) error {
	obj, err := readJSONFile(path)
	if err != nil {
		return err
	}

	servers, _ := obj["cursor.mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["alogin"] = map[string]any{
		"command": binPath,
		"args":    []string{"agent", "mcp"},
	}
	obj["cursor.mcpServers"] = servers

	return writeJSONFile(path, obj)
}

// mergeClaudeDesktopConfig writes `mcpServers.alogin` into claude_desktop_config.json.
func mergeClaudeDesktopConfig(path, binPath string) error {
	obj, err := readJSONFile(path)
	if err != nil {
		return err
	}

	servers, _ := obj["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["alogin"] = map[string]any{
		"command": binPath,
		"args":    []string{"agent", "mcp"},
	}
	obj["mcpServers"] = servers

	if err := writeJSONFile(path, obj); err != nil {
		return err
	}
	fmt.Println("  Note: restart Claude Desktop for the change to take effect.")
	return nil
}

// mergeVSCodeConfig writes `mcp.servers.alogin` into VS Code's settings.json.
func mergeVSCodeConfig(path, binPath string) error {
	obj, err := readJSONFile(path)
	if err != nil {
		return err
	}

	mcpBlock, _ := obj["mcp"].(map[string]any)
	if mcpBlock == nil {
		mcpBlock = map[string]any{}
	}
	servers, _ := mcpBlock["servers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["alogin"] = map[string]any{
		"type":    "stdio",
		"command": binPath,
		"args":    []string{"agent", "mcp"},
	}
	mcpBlock["servers"] = servers
	obj["mcp"] = mcpBlock

	return writeJSONFile(path, obj)
}

// ── JSON file helpers ─────────────────────────────────────────────────────────

func readJSONFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return obj, nil
}

func writeJSONFile(path string, obj map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

// ── newAgentSkillsCmd ─────────────────────────────────────────────────────────

func newAgentSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage alogin agent skills",
		Long: `Manage alogin skills installed to ~/.agent/skills.

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
				dir = filepath.Join(homeDir(), ".agent", "skills")
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
	cmd.Flags().StringVar(&dir, "dir", "", "Install destination (default: ~/.agent/skills)")
	return cmd
}

func newAgentSkillsListCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed alogin skills",
		Annotations: map[string]string{skipDBAnnotation: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				dir = filepath.Join(homeDir(), ".agent", "skills")
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
	cmd.Flags().StringVar(&dir, "dir", "", "Skills directory to list (default: ~/.agent/skills)")
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

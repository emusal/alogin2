package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	pluginpkg "github.com/emusal/alogin2/internal/plugin"
	tunnelpkg "github.com/emusal/alogin2/internal/tunnel"
)

// Update implements tea.Model — handles all key events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Non-key messages first
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		return m, nil
	case statsMsg:
		m.gateways = msg.gateways
		m.clusters = msg.clusters
		m.gwCount = len(msg.gateways)
		m.clCount = len(msg.clusters)
		return m, nil
	case gwLoadedMsg:
		m.gateways = msg.gateways
		return m, nil
	case formDoneMsg:
		m.servers = msg.servers
		m.applyFilter()
		m.state = stateList
		m.query = ""
		m.statusMsg = "Saved."
		return m, nil
	case formErrMsg:
		m.statusMsg = "Error: " + msg.err.Error()
		return m, nil
	case gwDoneMsg:
		m.gateways = msg.gateways
		m.state = stateGatewayList
		if msg.msg != "" {
			m.statusMsg = msg.msg
		}
		return m, nil
	case gwErrMsg:
		m.statusMsg = "Error: " + msg.err.Error()
		return m, nil
	case clDoneMsg:
		m.clusters = msg.clusters
		m.state = stateClusterList
		if msg.msg != "" {
			m.statusMsg = msg.msg
		}
		return m, nil
	case clErrMsg:
		m.statusMsg = "Error: " + msg.err.Error()
		return m, nil
	case hostDoneMsg:
		m.localHosts = msg.hosts
		m.state = stateHostList
		if msg.msg != "" {
			m.statusMsg = msg.msg
		}
		return m, nil
	case hostErrMsg:
		m.statusMsg = "Error: " + msg.err.Error()
		return m, nil
	case tnDoneMsg:
		m.tunnels = msg.tunnels
		m.state = stateTunnelList
		if msg.msg != "" {
			m.statusMsg = msg.msg
		}
		return m, m.loadTunnelStatusCmd()
	case tnErrMsg:
		m.statusMsg = "Error: " + msg.err.Error()
		return m, nil
	case tnStatusMsg:
		m.tnStatuses = msg.statuses
		return m, nil
	case pluginLoadedMsg:
		m.plugins = msg.plugins
		return m, nil
	case pluginListLoadedMsg:
		m.pluginList = msg.plugins
		m.pluginListCursor = 0
		m.state = statePluginList
		return m, nil
	case pluginReloadedMsg:
		m.pluginList = msg.plugins
		m.pluginDetail = msg.detail
		m.state = statePluginDetail
		m.statusMsg = "Saved."
		return m, nil
	case asDoneMsg:
		m.appServers = msg.appServers
		m.state = stateAppServerList
		if msg.msg != "" {
			m.statusMsg = msg.msg
		}
		return m, nil
	case asErrMsg:
		m.statusMsg = "Error: " + msg.err.Error()
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch m.state {
	case stateWelcome:
		return m.updateWelcome(keyMsg)
	case stateList, stateDetail:
		return m.updateList(keyMsg)
	case stateServerForm:
		return m.updateServerForm(keyMsg)
	case stateConfirmDelete:
		return m.updateConfirmDelete(keyMsg)
	case stateGatewayList:
		return m.updateGatewayList(keyMsg)
	case stateGatewayForm:
		return m.updateGatewayForm(keyMsg)
	case stateClusterList:
		return m.updateClusterList(keyMsg)
	case stateClusterForm:
		return m.updateClusterForm(keyMsg)
	case stateHostList:
		return m.updateHostList(keyMsg)
	case stateHostForm:
		return m.updateHostForm(keyMsg)
	case stateTunnelList:
		return m.updateTunnelList(keyMsg)
	case stateTunnelForm:
		return m.updateTunnelForm(keyMsg)
	case statePluginPicker:
		return m.updatePluginPicker(keyMsg)
	case statePluginList:
		return m.updatePluginList(keyMsg)
	case statePluginDetail:
		return m.updatePluginDetail(keyMsg)
	case stateAppServerList:
		return m.updateAppServerList(keyMsg)
	case stateAppServerForm:
		return m.updateAppServerForm(keyMsg)
	}
	return m, nil
}

// ── welcome screen ────────────────────────────────────────────────────────────

func (m Model) updateWelcome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		m.quitting = true
		return m, tea.Quit
	case "enter":
		// Show server list
		m.state = stateList
		return m, nil
	default:
		// Any other key: transition to list and forward the key press
		m.state = stateList
		return m.updateList(msg)
	}
}

// ── main list / detail ────────────────────────────────────────────────────────

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Detail overlay has its own bindings
	if m.state == stateDetail {
		return m.updateDetail(msg)
	}

	// Slash-command palette active
	if strings.HasPrefix(m.query, "/") {
		return m.updateCommandPalette(msg)
	}

	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "q":
		if m.query != "" {
			m.query = ""
			m.applyFilter()
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case "esc":
		if m.query != "" {
			m.query = ""
			m.applyFilter()
			return m, nil
		}
		m.state = stateWelcome
		return m, nil

	case "enter":
		if len(m.filtered) == 0 {
			return m, nil
		}
		srv := m.filtered[m.cursor]
		m.choice = &SelectedServer{Server: srv, User: srv.User}
		return m, tea.Quit

	case "r":
		if m.query == "" && len(m.filtered) > 0 {
			srv := m.filtered[m.cursor]
			m.choice = &SelectedServer{Server: srv, User: srv.User, AutoGW: true}
			return m, tea.Quit
		}
		// fall through to default (append 'r' to search)
		m.query += "r"
		m.applyFilter()
		return m, nil

	case "tab":
		if len(m.filtered) > 0 {
			m.state = stateDetail
		}
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}

	case "backspace":
		if len(m.query) > 0 {
			m.query = m.query[:len(m.query)-1]
			m.applyFilter()
		}

	default:
		if len(msg.Runes) == 1 {
			ch := msg.Runes[0]
			// CRUD shortcuts when query is empty
			if m.query == "" {
				switch ch {
				case 'a':
					return m, m.initServerForm(nil)
				case 'e':
					if len(m.filtered) > 0 {
						return m, m.initServerForm(m.filtered[m.cursor])
					}
					return m, nil
				case 'd':
					if len(m.filtered) > 0 {
						m.deleteTarget = m.filtered[m.cursor]
						m.state = stateConfirmDelete
					}
					return m, nil
				case 'p':
					if len(m.filtered) > 0 && m.pluginDir != "" {
						m.pluginCursor = 0
						m.state = statePluginPicker
						return m, m.loadPluginsCmd()
					}
					return m, nil
				}
			}
			m.statusMsg = ""
			m.query += string(ch)
			m.cmdCursor = 0
			m.applyFilter()
		}
	}
	return m, nil
}

func (m Model) updateCommandPalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cmds := m.filteredCommands()

	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "esc":
		m.query = ""
		m.cmdCursor = 0
		m.applyFilter()
		return m, nil

	case "backspace":
		if len(m.query) > 0 {
			m.query = m.query[:len(m.query)-1]
			m.cmdCursor = 0
			m.applyFilter()
		}
		return m, nil

	case "up", "k":
		if m.cmdCursor > 0 {
			m.cmdCursor--
		}

	case "down", "j":
		if m.cmdCursor < len(cmds)-1 {
			m.cmdCursor++
		}

	case "tab":
		// Autocomplete to the selected command
		if len(cmds) > 0 {
			m.query = cmds[m.cmdCursor].trigger
			m.cmdCursor = 0
		}

	case "enter":
		if len(cmds) == 0 {
			return m, nil
		}
		cmd := cmds[m.cmdCursor]
		m.query = ""
		m.cmdCursor = 0
		return m.executeCommand(cmd.trigger)

	default:
		if len(msg.Runes) == 1 {
			m.query += string(msg.Runes[0])
			m.cmdCursor = 0
		}
	}
	return m, nil
}

func (m Model) executeCommand(trigger string) (tea.Model, tea.Cmd) {
	m.statusMsg = ""
	switch trigger {
	case "/compute":
		m.state = stateList
		return m, nil
	case "/gateway":
		return m, m.loadGatewaysCmd()
	case "/cluster":
		return m, m.loadClustersCmd()
	case "/hosts":
		return m, m.loadHostsCmd()
	case "/tunnel":
		return m, m.loadTunnelsCmd()
	case "/app-server":
		return m, m.loadAppServersCmd()
	case "/plugin":
		return m, m.loadPluginListCmd()
	}
	return m, nil
}

func (m Model) updatePluginList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.state = stateList
		return m, nil
	case "up", "k":
		if m.pluginListCursor > 0 {
			m.pluginListCursor--
		}
	case "down", "j":
		if m.pluginListCursor < len(m.pluginList)-1 {
			m.pluginListCursor++
		}
	case "enter":
		if len(m.pluginList) > 0 {
			m.pluginDetail = m.pluginList[m.pluginListCursor]
			m.state = statePluginDetail
			m.statusMsg = ""
		}
	}
	return m, nil
}

func (m Model) updatePluginDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.state = statePluginList
		m.statusMsg = ""
	case "e":
		if m.pluginDetail == nil || m.pluginDir == "" {
			return m, nil
		}
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = os.Getenv("VISUAL")
		}
		if editor == "" {
			m.statusMsg = "Set $EDITOR to enable editing (e.g. export EDITOR=vim)"
			return m, nil
		}
		// FilePath is set by LoadFromFile — stable regardless of the name field inside the YAML.
		filePath := m.pluginDetail.FilePath
		dir := m.pluginDir
		return m, tea.ExecProcess(
			exec.Command(editor, filePath),
			func(err error) tea.Msg {
				if err != nil {
					return pluginReloadedMsg{}
				}
				list, _ := pluginpkg.LoadDir(dir)
				detail, _ := pluginpkg.LoadFromFile(filePath)
				return pluginReloadedMsg{plugins: list, detail: detail}
			},
		)
	}
	return m, nil
}

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc", "tab":
		m.state = stateList
	case "enter":
		if len(m.filtered) > 0 {
			srv := m.filtered[m.cursor]
			m.choice = &SelectedServer{Server: srv, User: srv.User}
			return m, tea.Quit
		}
	case "r":
		if len(m.filtered) > 0 {
			srv := m.filtered[m.cursor]
			m.choice = &SelectedServer{Server: srv, User: srv.User, AutoGW: true}
			return m, tea.Quit
		}
	case "e":
		if len(m.filtered) > 0 {
			return m, m.initServerForm(m.filtered[m.cursor])
		}
	case "d":
		if len(m.filtered) > 0 {
			m.deleteTarget = m.filtered[m.cursor]
			m.state = stateConfirmDelete
		}
	}
	return m, nil
}

// ── server form / delete confirm ──────────────────────────────────────────────

// tabCount is the total number of Tab stops in the server form:
// 0..4 = text fields, 5 = gateway picker row, 6 = locale field.
const srvFormTabCount = 7

func (m Model) updateServerForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Gateway picker is open — handle search + navigation
	if m.srvFormGwPickerOpen {
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			m.srvFormGwPickerOpen = false
			m.srvFormGwSearch.Blur()
			return m, nil
		case "up":
			if m.srvFormGwPickerCursor > 0 {
				m.srvFormGwPickerCursor--
			}
		case "down":
			entries := m.gwPickerEntries()
			if m.srvFormGwPickerCursor < len(entries)-1 {
				m.srvFormGwPickerCursor++
			}
		case "enter":
			entries := m.gwPickerEntries()
			if len(entries) > 0 {
				e := entries[m.srvFormGwPickerCursor]
				m.srvFormSelectedGwID = e.gwID
				m.srvFormSelectedSrvGwID = e.srvGwID
			}
			m.srvFormGwPickerOpen = false
			m.srvFormGwSearch.SetValue("")
			m.srvFormGwSearch.Blur()
		default:
			// Forward all other keys to the search input
			var cmd tea.Cmd
			m.srvFormGwSearch, cmd = m.srvFormGwSearch.Update(msg)
			// Clamp cursor after filter change
			entries := m.gwPickerEntries()
			if m.srvFormGwPickerCursor >= len(entries) {
				m.srvFormGwPickerCursor = len(entries) - 1
			}
			if m.srvFormGwPickerCursor < 0 {
				m.srvFormGwPickerCursor = 0
			}
			return m, cmd
		}
		return m, nil
	}

	// Normal form navigation
	// formFocusIdx: 0-4 = text fields, 5 = gateway row, 6 = locale text field
	blurCurrent := func() {
		if m.formFocusIdx != 5 {
			idx := m.formFocusIdx
			if idx > 5 {
				idx-- // formFields[5] is Locale (Tab-index 6)
			}
			m.formFields[idx].Blur()
		}
	}
	focusCurrent := func() {
		if m.formFocusIdx != 5 {
			idx := m.formFocusIdx
			if idx > 5 {
				idx--
			}
			m.formFields[idx].Focus()
		}
	}

	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.state = stateList
		return m, nil
	case "enter":
		if m.formFocusIdx == 5 {
			// Open gateway picker
			m.srvFormGwPickerOpen = true
			m.srvFormGwPickerCursor = 0
			m.srvFormGwSearch.SetValue("")
			m.srvFormGwSearch.Focus()
			return m, nil
		}
		return m, m.submitServerForm()
	case "tab":
		blurCurrent()
		next := (m.formFocusIdx + 1) % srvFormTabCount
		if m.formMode == fmEdit && next == 1 {
			next = 2
		}
		m.formFocusIdx = next
		focusCurrent()
		return m, nil
	case "shift+tab":
		blurCurrent()
		prev := (m.formFocusIdx - 1 + srvFormTabCount) % srvFormTabCount
		if m.formMode == fmEdit && prev == 1 {
			prev = 0
		}
		m.formFocusIdx = prev
		focusCurrent()
		return m, nil
	default:
		if m.formFocusIdx == 5 {
			return m, nil
		}
		idx := m.formFocusIdx
		if idx > 5 {
			idx--
		}
		var cmd tea.Cmd
		m.formFields[idx], cmd = m.formFields[idx].Update(msg)
		return m, cmd
	}
}

func (m Model) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "y", "Y":
		return m, m.deleteServerCmd()
	case "n", "N", "esc":
		m.state = stateList
	}
	return m, nil
}

// ── gateway list / form ───────────────────────────────────────────────────────

func (m Model) updateGatewayList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.state = stateWelcome
	case "up", "k":
		if m.gatewayCursor > 0 {
			m.gatewayCursor--
		}
	case "down", "j":
		if m.gatewayCursor < len(m.gateways)-1 {
			m.gatewayCursor++
		}
	case "a":
		m.initGatewayForm(nil)
	case "e":
		if len(m.gateways) > 0 {
			m.initGatewayForm(m.gateways[m.gatewayCursor])
		}
	case "d":
		if len(m.gateways) > 0 {
			gw := m.gateways[m.gatewayCursor]
			if m.gatewayCursor >= len(m.gateways)-1 && m.gatewayCursor > 0 {
				m.gatewayCursor--
			}
			return m, m.deleteGatewayCmd(gw.ID)
		}
	}
	return m, nil
}

func (m Model) updateGatewayForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "ctrl+s":
		return m, m.submitGatewayForm()
	case "esc":
		if m.gwFormPickerOpen {
			m.gwFormPickerOpen = false
			return m, nil
		}
		m.state = stateGatewayList
		return m, nil
	case "tab":
		if m.gwFormName.Focused() {
			m.gwFormName.Blur()
		} else {
			m.gwFormName.Focus()
		}
		return m, nil
	}

	if m.gwFormName.Focused() {
		var cmd tea.Cmd
		m.gwFormName, cmd = m.gwFormName.Update(msg)
		return m, cmd
	}

	if m.gwFormPickerOpen {
		switch msg.String() {
		case "up", "k":
			if m.gwFormPickerCursor > 0 {
				m.gwFormPickerCursor--
			}
		case "down", "j":
			if m.gwFormPickerCursor < len(m.servers)-1 {
				m.gwFormPickerCursor++
			}
		case "enter":
			if len(m.servers) > 0 {
				m.gwFormHops = append(m.gwFormHops, m.servers[m.gwFormPickerCursor].ID)
			}
			m.gwFormPickerOpen = false
		}
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.gwFormHopCursor > 0 {
			m.gwFormHopCursor--
		}
	case "down", "j":
		if m.gwFormHopCursor < len(m.gwFormHops)-1 {
			m.gwFormHopCursor++
		}
	case "a", "enter":
		m.gwFormPickerOpen = true
		m.gwFormPickerCursor = 0
	case "x", "backspace":
		if len(m.gwFormHops) > 0 {
			idx := m.gwFormHopCursor
			m.gwFormHops = append(m.gwFormHops[:idx], m.gwFormHops[idx+1:]...)
			if m.gwFormHopCursor >= len(m.gwFormHops) && m.gwFormHopCursor > 0 {
				m.gwFormHopCursor--
			}
		}
	case "u":
		i := m.gwFormHopCursor
		if i > 0 {
			m.gwFormHops[i-1], m.gwFormHops[i] = m.gwFormHops[i], m.gwFormHops[i-1]
			m.gwFormHopCursor--
		}
	case "m":
		i := m.gwFormHopCursor
		if i < len(m.gwFormHops)-1 {
			m.gwFormHops[i], m.gwFormHops[i+1] = m.gwFormHops[i+1], m.gwFormHops[i]
			m.gwFormHopCursor++
		}
	}
	return m, nil
}

// ── cluster list / form ───────────────────────────────────────────────────────

func (m Model) updateClusterList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.state = stateWelcome
	case "up", "k":
		if m.clCursor > 0 {
			m.clCursor--
		}
	case "down", "j":
		if m.clCursor < len(m.clusters)-1 {
			m.clCursor++
		}
	case "a":
		m.initClusterForm(nil)
	case "e":
		if len(m.clusters) > 0 {
			m.initClusterForm(m.clusters[m.clCursor])
		}
	case "d":
		if len(m.clusters) > 0 {
			cl := m.clusters[m.clCursor]
			if m.clCursor >= len(m.clusters)-1 && m.clCursor > 0 {
				m.clCursor--
			}
			return m, m.deleteClusterCmd(cl.ID)
		}
	}
	return m, nil
}

func (m Model) updateClusterForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "ctrl+s":
		return m, m.submitClusterForm()
	case "esc":
		if m.clFormUserEditOpen {
			m.clFormUserEditOpen = false
			return m, nil
		}
		if m.clFormPickerOpen {
			m.clFormPickerOpen = false
			return m, nil
		}
		m.state = stateClusterList
		return m, nil
	case "tab":
		if m.clFormName.Focused() {
			m.clFormName.Blur()
		} else {
			m.clFormName.Focus()
		}
		return m, nil
	}

	if m.clFormName.Focused() {
		var cmd tea.Cmd
		m.clFormName, cmd = m.clFormName.Update(msg)
		return m, cmd
	}

	// User override input active
	if m.clFormUserEditOpen {
		switch msg.String() {
		case "enter":
			if m.clFormMemberCursor < len(m.clFormMembers) {
				m.clFormMembers[m.clFormMemberCursor].user = m.clFormUserInput.Value()
			}
			m.clFormUserEditOpen = false
		default:
			var cmd tea.Cmd
			m.clFormUserInput, cmd = m.clFormUserInput.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	// Server picker
	if m.clFormPickerOpen {
		switch msg.String() {
		case "up", "k":
			if m.clFormPickerCursor > 0 {
				m.clFormPickerCursor--
			}
		case "down", "j":
			if m.clFormPickerCursor < len(m.servers)-1 {
				m.clFormPickerCursor++
			}
		case "enter":
			if len(m.servers) > 0 {
				m.clFormMembers = append(m.clFormMembers,
					memberEntry{serverID: m.servers[m.clFormPickerCursor].ID})
			}
			m.clFormPickerOpen = false
		}
		return m, nil
	}

	// Member list navigation
	switch msg.String() {
	case "up", "k":
		if m.clFormMemberCursor > 0 {
			m.clFormMemberCursor--
		}
	case "down", "j":
		if m.clFormMemberCursor < len(m.clFormMembers)-1 {
			m.clFormMemberCursor++
		}
	case "a", "enter":
		m.clFormPickerOpen = true
		m.clFormPickerCursor = 0
	case "x", "backspace":
		if len(m.clFormMembers) > 0 {
			idx := m.clFormMemberCursor
			m.clFormMembers = append(m.clFormMembers[:idx], m.clFormMembers[idx+1:]...)
			if m.clFormMemberCursor >= len(m.clFormMembers) && m.clFormMemberCursor > 0 {
				m.clFormMemberCursor--
			}
		}
	case "u":
		i := m.clFormMemberCursor
		if i > 0 {
			m.clFormMembers[i-1], m.clFormMembers[i] = m.clFormMembers[i], m.clFormMembers[i-1]
			m.clFormMemberCursor--
		}
	case "m":
		i := m.clFormMemberCursor
		if i < len(m.clFormMembers)-1 {
			m.clFormMembers[i], m.clFormMembers[i+1] = m.clFormMembers[i+1], m.clFormMembers[i]
			m.clFormMemberCursor++
		}
	case "r":
		// Edit user override for selected member
		if len(m.clFormMembers) > 0 {
			cur := m.clFormMembers[m.clFormMemberCursor].user
			m.clFormUserInput.SetValue(cur)
			m.clFormUserInput.Focus()
			m.clFormUserEditOpen = true
		}
	}
	return m, nil
}

// ── local hosts list / form ───────────────────────────────────────────────────

func (m Model) updateHostList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.state = stateWelcome
	case "up", "k":
		if m.hostCursor > 0 {
			m.hostCursor--
		}
	case "down", "j":
		if m.hostCursor < len(m.localHosts)-1 {
			m.hostCursor++
		}
	case "a":
		m.initHostForm(nil)
	case "e":
		if len(m.localHosts) > 0 {
			m.initHostForm(m.localHosts[m.hostCursor])
		}
	case "d":
		if len(m.localHosts) > 0 {
			h := m.localHosts[m.hostCursor]
			if m.hostCursor >= len(m.localHosts)-1 && m.hostCursor > 0 {
				m.hostCursor--
			}
			return m, m.deleteHostCmd(h.ID)
		}
	}
	return m, nil
}

func (m Model) updateHostForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.state = stateHostList
		return m, nil
	case "ctrl+s", "enter":
		if m.hostFormFocus == len(m.hostFormFields)-1 || msg.String() == "ctrl+s" {
			return m, m.submitHostForm()
		}
		// Tab to next field on enter when not on last field
		m.hostFormFields[m.hostFormFocus].Blur()
		m.hostFormFocus = (m.hostFormFocus + 1) % len(m.hostFormFields)
		m.hostFormFields[m.hostFormFocus].Focus()
		return m, nil
	case "tab":
		m.hostFormFields[m.hostFormFocus].Blur()
		m.hostFormFocus = (m.hostFormFocus + 1) % len(m.hostFormFields)
		// Skip hostname field when editing
		if m.hostFormMode == fmEdit && m.hostFormFocus == 0 {
			m.hostFormFocus = 1
		}
		m.hostFormFields[m.hostFormFocus].Focus()
		return m, nil
	case "shift+tab":
		m.hostFormFields[m.hostFormFocus].Blur()
		m.hostFormFocus = (m.hostFormFocus - 1 + len(m.hostFormFields)) % len(m.hostFormFields)
		if m.hostFormMode == fmEdit && m.hostFormFocus == 0 {
			m.hostFormFocus = len(m.hostFormFields) - 1
		}
		m.hostFormFields[m.hostFormFocus].Focus()
		return m, nil
	default:
		var cmd tea.Cmd
		m.hostFormFields[m.hostFormFocus], cmd = m.hostFormFields[m.hostFormFocus].Update(msg)
		return m, cmd
	}
}

// ── tunnel list / form ────────────────────────────────────────────────────────

func (m Model) updateTunnelList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.state = stateWelcome
	case "up", "k":
		if m.tunnelCursor > 0 {
			m.tunnelCursor--
		}
	case "down", "j":
		if m.tunnelCursor < len(m.tunnels)-1 {
			m.tunnelCursor++
		}
	case "a":
		m.initTunnelForm(nil)
	case "e":
		if len(m.tunnels) > 0 {
			m.initTunnelForm(m.tunnels[m.tunnelCursor])
		}
	case "d":
		if len(m.tunnels) > 0 {
			t := m.tunnels[m.tunnelCursor]
			if m.tunnelCursor >= len(m.tunnels)-1 && m.tunnelCursor > 0 {
				m.tunnelCursor--
			}
			return m, m.deleteTunnelCmd(t.ID)
		}
	case "s":
		// Start selected tunnel
		if len(m.tunnels) > 0 {
			t := m.tunnels[m.tunnelCursor]
			return m, m.tunnelStartCmd(t.Name)
		}
	case "x":
		// Stop selected tunnel
		if len(m.tunnels) > 0 {
			t := m.tunnels[m.tunnelCursor]
			return m, m.tunnelStopCmd(t.Name)
		}
	}
	return m, nil
}

func (m Model) tunnelStartCmd(name string) tea.Cmd {
	return func() tea.Msg {
		binPath, err := os.Executable()
		if err != nil {
			return tnErrMsg{err}
		}
		if err := tunnelpkg.Start(name, binPath); err != nil {
			return tnErrMsg{err}
		}
		tunnels, _ := m.db.Tunnels.ListAll(context.Background())
		return tnDoneMsg{tunnels, "Started."}
	}
}

func (m Model) tunnelStopCmd(name string) tea.Cmd {
	return func() tea.Msg {
		if err := tunnelpkg.Stop(name); err != nil {
			return tnErrMsg{err}
		}
		tunnels, _ := m.db.Tunnels.ListAll(context.Background())
		return tnDoneMsg{tunnels, "Stopped."}
	}
}

// tabCount for tunnel form: 7 text fields + 1 auto_gw toggle = 8 stops
const tnFormTabCount = 8

func (m Model) updateTunnelForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.state = stateTunnelList
		return m, nil
	case "ctrl+s":
		return m, m.submitTunnelForm()
	case "space":
		// Toggle AutoGW when focused on last tab stop (index 7)
		if m.tnFormFocus == tnFormTabCount-1 {
			m.tnFormAutoGW = !m.tnFormAutoGW
			return m, nil
		}
	case "enter":
		// Submit from last field
		if m.tnFormFocus == tnFormTabCount-1 {
			return m, m.submitTunnelForm()
		}
		// Otherwise move to next field
		m.tnFormFields[m.tnFormFocus].Blur()
		m.tnFormFocus = (m.tnFormFocus + 1) % tnFormTabCount
		if m.tnFormFocus < len(m.tnFormFields) {
			m.tnFormFields[m.tnFormFocus].Focus()
		}
		return m, nil
	case "tab":
		if m.tnFormFocus < len(m.tnFormFields) {
			m.tnFormFields[m.tnFormFocus].Blur()
		}
		m.tnFormFocus = (m.tnFormFocus + 1) % tnFormTabCount
		if m.tnFormFocus < len(m.tnFormFields) {
			m.tnFormFields[m.tnFormFocus].Focus()
		}
		return m, nil
	case "shift+tab":
		if m.tnFormFocus < len(m.tnFormFields) {
			m.tnFormFields[m.tnFormFocus].Blur()
		}
		m.tnFormFocus = (m.tnFormFocus - 1 + tnFormTabCount) % tnFormTabCount
		if m.tnFormFocus < len(m.tnFormFields) {
			m.tnFormFields[m.tnFormFocus].Focus()
		}
		return m, nil
	}
	// Forward key to active text field
	if m.tnFormFocus < len(m.tnFormFields) {
		var cmd tea.Cmd
		m.tnFormFields[m.tnFormFocus], cmd = m.tnFormFields[m.tnFormFocus].Update(msg)
		return m, cmd
	}
	return m, nil
}

// clFormPickerCursor clamp helper (used in render)
var _ = strings.TrimSpace // keep import

// ── plugin picker ─────────────────────────────────────────────────────────────

// updatePluginPicker handles key events for the plugin picker overlay.
func (m Model) updatePluginPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.state = stateList
		return m, nil
	case "up", "k":
		if m.pluginCursor > 0 {
			m.pluginCursor--
		}
	case "down", "j":
		if m.pluginCursor < len(m.plugins)-1 {
			m.pluginCursor++
		}
	case "enter":
		if len(m.plugins) == 0 {
			return m, nil
		}
		if len(m.filtered) == 0 {
			return m, nil
		}
		srv := m.filtered[m.cursor]
		m.choice = &SelectedServer{
			Server: srv,
			User:   srv.User,
			Plugin: m.plugins[m.pluginCursor],
		}
		return m, tea.Quit
	}
	return m, nil
}

// ── app-server list / form ────────────────────────────────────────────────────

func (m Model) updateAppServerList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.state = stateWelcome
	case "up", "k":
		if m.appServerCursor > 0 {
			m.appServerCursor--
		}
	case "down", "j":
		if m.appServerCursor < len(m.appServers)-1 {
			m.appServerCursor++
		}
	case "a":
		m.initAppServerForm(nil)
	case "e":
		if len(m.appServers) > 0 {
			m.initAppServerForm(m.appServers[m.appServerCursor])
		}
	case "d":
		if len(m.appServers) > 0 {
			as := m.appServers[m.appServerCursor]
			if m.appServerCursor >= len(m.appServers)-1 && m.appServerCursor > 0 {
				m.appServerCursor--
			}
			return m, m.deleteAppServerCmd(as.ID)
		}
	case "enter":
		// Connect via selected app-server binding
		if len(m.appServers) > 0 {
			as := m.appServers[m.appServerCursor]
			srv := serverByID(m.servers, as.ServerID)
			if srv == nil {
				m.statusMsg = fmt.Sprintf("server id=%d not found", as.ServerID)
				return m, nil
			}
			m.choice = &SelectedServer{
				Server: srv,
				User:   srv.User,
				AutoGW: as.AutoGW,
				Plugin: as.PluginName,
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

// asFormTabCount: 3 text fields (name, plugin, desc) + 1 server picker row + 1 auto_gw toggle = 5 stops
const asFormTabCount = 5

// asFormIdxServer and asFormIdxAutoGW are the tab-stop indices for the non-text-field rows.
const (
	asFormIdxServer = 3
	asFormIdxAutoGW = 4
)

func (m Model) updateAppServerForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle server picker overlay first
	if m.asFormPickerOpen {
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			m.asFormPickerOpen = false
		case "up", "k":
			if m.asFormPickerCursor > 0 {
				m.asFormPickerCursor--
			}
		case "down", "j":
			if m.asFormPickerCursor < len(m.servers)-1 {
				m.asFormPickerCursor++
			}
		case "enter":
			if len(m.servers) > 0 {
				m.asFormServerID = m.servers[m.asFormPickerCursor].ID
			}
			m.asFormPickerOpen = false
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.state = stateAppServerList
		return m, nil
	case "ctrl+s":
		return m, m.submitAppServerForm()
	case "space":
		if m.asFormFocus == asFormIdxAutoGW {
			m.asFormAutoGW = !m.asFormAutoGW
			return m, nil
		}
	case "enter":
		if m.asFormFocus == asFormIdxServer {
			m.asFormPickerOpen = true
			m.asFormPickerCursor = 0
			return m, nil
		}
		if m.asFormFocus == asFormIdxAutoGW {
			m.asFormAutoGW = !m.asFormAutoGW
			return m, nil
		}
		// Move to next field
		if m.asFormFocus < len(m.asFormFields) {
			m.asFormFields[m.asFormFocus].Blur()
		}
		m.asFormFocus = (m.asFormFocus + 1) % asFormTabCount
		if m.asFormFocus < len(m.asFormFields) {
			m.asFormFields[m.asFormFocus].Focus()
		}
		return m, nil
	case "tab":
		if m.asFormFocus < len(m.asFormFields) {
			m.asFormFields[m.asFormFocus].Blur()
		}
		m.asFormFocus = (m.asFormFocus + 1) % asFormTabCount
		if m.asFormFocus < len(m.asFormFields) {
			m.asFormFields[m.asFormFocus].Focus()
		}
		return m, nil
	case "shift+tab":
		if m.asFormFocus < len(m.asFormFields) {
			m.asFormFields[m.asFormFocus].Blur()
		}
		m.asFormFocus = (m.asFormFocus - 1 + asFormTabCount) % asFormTabCount
		if m.asFormFocus < len(m.asFormFields) {
			m.asFormFields[m.asFormFocus].Focus()
		}
		return m, nil
	}

	// Forward key to active text field
	if m.asFormFocus < len(m.asFormFields) {
		var cmd tea.Cmd
		m.asFormFields[m.asFormFocus], cmd = m.asFormFields[m.asFormFocus].Update(msg)
		return m, cmd
	}
	return m, nil
}

// loadPluginsCmd returns a tea.Cmd that reads plugin names from the plugins directory.
func (m Model) loadPluginsCmd() tea.Cmd {
	dir := m.pluginDir
	return func() tea.Msg {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return pluginLoadedMsg{plugins: nil}
		}
		var names []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasSuffix(name, ".yaml") {
				names = append(names, strings.TrimSuffix(name, ".yaml"))
			}
		}
		return pluginLoadedMsg{plugins: names}
	}
}

package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
	case "/server":
		m.state = stateList
		return m, nil
	case "/gateway":
		return m, m.loadGatewaysCmd()
	case "/cluster":
		return m, m.loadClustersCmd()
	case "/hosts":
		return m, m.loadHostsCmd()
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

// clFormPickerCursor clamp helper (used in render)
var _ = strings.TrimSpace // keep import

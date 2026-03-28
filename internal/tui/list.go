package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/emusal/alogin2/internal/model"
	"github.com/sahilm/fuzzy"
)

// applyFilter updates m.filtered using fuzzy search on m.query.
func (m *Model) applyFilter() {
	m.cursor = 0
	q := m.query
	if strings.HasPrefix(q, "/") {
		// slash-command mode — don't filter server list
		m.filtered = m.servers
		return
	}
	if q == "" {
		m.filtered = m.servers
		return
	}
	sources := make([]string, len(m.servers))
	for i, s := range m.servers {
		sources[i] = s.Host + " " + s.User + " " + string(s.Protocol)
	}
	matches := fuzzy.Find(q, sources)
	filtered := make([]*model.Server, 0, len(matches))
	for _, match := range matches {
		filtered = append(filtered, m.servers[match.Index])
	}
	m.filtered = filtered
}

// ── top-level View ────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	switch m.state {
	case stateWelcome:
		return m.renderWelcome()
	case stateServerForm:
		return m.renderServerForm()
	case stateConfirmDelete:
		return m.renderConfirmDelete()
	case stateGatewayList:
		return m.renderGatewayList()
	case stateGatewayForm:
		return m.renderGatewayForm()
	case stateClusterList:
		return m.renderClusterList()
	case stateClusterForm:
		return m.renderClusterForm()
	case stateHostList:
		return m.renderHostList()
	case stateHostForm:
		return m.renderHostForm()
	case stateTunnelList:
		return m.renderTunnelList()
	case stateTunnelForm:
		return m.renderTunnelForm()
	case statePluginPicker:
		return m.renderPluginPicker()
	case statePluginList:
		return m.renderPluginList()
	case statePluginDetail:
		return m.renderPluginDetail()
	case stateAppServerList:
		return m.renderAppServerList()
	case stateAppServerForm:
		return m.renderAppServerForm()
	}
	return m.renderMainList()
}

// ── welcome screen ────────────────────────────────────────────────────────────

func (m Model) renderWelcome() string {
	var sb strings.Builder

	// Determine box width from terminal size
	boxW := 56
	if m.termWidth > 0 {
		boxW = min(m.termWidth-6, 64)
	}

	// ── header box ──
	headerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("99")).
		Padding(1, 4).
		Width(boxW)

	logoLine := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("alogin") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  ·  SSH Connection Manager")
	versionLine := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Render("v" + m.version)

	sb.WriteString("\n")
	sb.WriteString(headerStyle.Render(logoLine + "\n" + versionLine))
	sb.WriteString("\n\n")

	// ── stats ──
	statsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	stats := fmt.Sprintf("  %d servers  ·  %d gateways  ·  %d clusters",
		len(m.servers), m.gwCount, m.clCount)
	sb.WriteString(statsStyle.Render(stats))
	sb.WriteString("\n\n\n")

	// ── input box ──
	inputBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("212")).
		Padding(0, 1).
		Width(boxW)

	promptStr := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render(">")
	phStr := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Search hosts or type / for commands...")
	inputLine := promptStr + " " + phStr + "▊"
	sb.WriteString(inputBoxStyle.Render(inputLine))
	sb.WriteString("\n\n")

	// ── hints ──
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	sb.WriteString(hintStyle.Render("  /gateway  ·  /cluster  ·  /compute  ·  /app-server  ·  a add  ·  q quit"))

	return sb.String()
}

// ── main list ─────────────────────────────────────────────────────────────────

func (m Model) renderMainList() string {
	var sb strings.Builder
	sb.WriteString(m.titleStyle.Render("alogin"))
	sb.WriteString("\n")
	l1, l2 := pageDesc("server")
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	sb.WriteString(descStyle.Render("  " + l1))
	sb.WriteString("\n")
	sb.WriteString(descStyle.Render("  " + l2))
	sb.WriteString("\n\n")

	// Input bar — Claude-style "> " prompt
	promptStr := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render(">")
	var inputLine string
	if m.state == stateDetail {
		inputLine = promptStr + " " + m.query
	} else if m.query == "" {
		ph := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Search hosts or type / for commands...")
		inputLine = promptStr + " " + ph + "▊"
	} else {
		inputLine = promptStr + " " + m.query + "▊"
	}
	sb.WriteString(m.inputStyle.Render(inputLine))
	sb.WriteString("\n")

	// Slash-command palette
	if strings.HasPrefix(m.query, "/") {
		sb.WriteString(m.renderCommandPalette())
		return sb.String()
	}

	// Detail overlay
	if m.state == stateDetail && len(m.filtered) > 0 {
		sb.WriteString(m.renderDetail(m.filtered[m.cursor]))
		return sb.String()
	}

	// Server list — viewport-clipped to terminal height
	// Fixed lines: title(1) desc(2) blank(1) inputBorder(3) blank(1) hint(1) blank(1) = 10
	viewport := m.visibleRows(10)
	total := len(m.filtered)
	viewStart, viewEnd := m.viewWindow(m.cursor, total, viewport)

	sb.WriteString("\n")
	if total == 0 {
		sb.WriteString(m.dimStyle.Render("  (no results)"))
	}
	for i := viewStart; i < viewEnd; i++ {
		s := m.filtered[i]
		gw := ""
		if s.GatewayID != nil {
			gw = " [gw]"
		}
		line := fmt.Sprintf("%-28s  %-16s  %-8s%s", s.Host, s.User, string(s.Protocol), gw)
		if i == m.cursor {
			sb.WriteString(m.selectedStyle.Render("> " + line))
		} else {
			sb.WriteString(m.normalStyle.Render("  " + line))
		}
		sb.WriteString("\n")
	}
	if total > viewport {
		sb.WriteString(m.dimStyle.Render(fmt.Sprintf("  %d/%d", m.cursor+1, total)))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	hint := "[↑↓] navigate  [Enter] connect  [r] via-gw  [Tab] details  [/] commands"
	if m.query == "" {
		hint += "  [a] add  [e] edit  [d] delete  [q] quit"
	} else {
		hint += "  [q] quit"
	}
	sb.WriteString(m.dimStyle.Render(hint))

	if m.statusMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(m.dimStyle.Render("  " + m.statusMsg))
	}
	return sb.String()
}

func (m Model) renderCommandPalette() string {
	var sb strings.Builder
	cmds := m.filteredCommands()

	sb.WriteString("\n")
	var lines strings.Builder
	if len(cmds) == 0 {
		lines.WriteString(m.dimStyle.Render("  (no matching commands)"))
	}
	for i, c := range cmds {
		line := fmt.Sprintf("%-12s  %s", c.trigger, c.desc)
		if i == m.cmdCursor {
			lines.WriteString(m.selectedStyle.Render("> " + line))
		} else {
			lines.WriteString(m.normalStyle.Render("  " + line))
		}
		lines.WriteString("\n")
	}
	sb.WriteString(m.cmdStyle.Render(lines.String()))
	sb.WriteString("\n")
	sb.WriteString(m.dimStyle.Render("  [↑↓] navigate  [Enter] open  [Tab] complete  [Esc] cancel"))
	return sb.String()
}

// ── server detail ─────────────────────────────────────────────────────────────

func (m Model) renderDetail(s *model.Server) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Host:     %s\n", s.Host))
	sb.WriteString(fmt.Sprintf("User:     %s\n", s.User))
	sb.WriteString(fmt.Sprintf("Protocol: %s\n", s.Protocol))
	port := "default"
	if s.Port > 0 {
		port = fmt.Sprintf("%d", s.Port)
	}
	sb.WriteString(fmt.Sprintf("Port:     %s\n", port))
	if s.Locale != "" && s.Locale != "-" {
		sb.WriteString(fmt.Sprintf("Locale:   %s\n", s.Locale))
	}
	if s.GatewayID != nil {
		for _, gw := range m.gateways {
			if gw.ID == *s.GatewayID {
				sb.WriteString(fmt.Sprintf("Gateway:  %s\n", gw.Name))
				break
			}
		}
	}
	sb.WriteString("\n")
	sb.WriteString("[Enter] connect  [r] via-gw  [e] edit  [d] delete  [Tab/Esc] back")
	return m.detailStyle.Render(sb.String())
}

// ── server form ───────────────────────────────────────────────────────────────

func (m Model) renderServerForm() string {
	var sb strings.Builder
	title := "Add Server"
	if m.formMode == fmEdit && m.formTarget != nil {
		title = fmt.Sprintf("Edit Server: %s", m.formTarget.Host)
	}
	sb.WriteString(m.titleStyle.Render("alogin — " + title))
	sb.WriteString("\n\n")

	// formFields: Protocol(0) Host(1) User(2) Password(3) Port(4) Locale(5)
	// formFocusIdx==5 is the virtual Gateway row (picker), ==6 is Locale
	textLabels := []string{"Protocol", "Host", "User", "Password", "Port"}
	for i, field := range m.formFields[:5] {
		label := textLabels[i]
		if m.formMode == fmEdit && i == 1 {
			label += " (locked)"
		}
		sb.WriteString(fmt.Sprintf("  %-12s  %s\n", label, field.View()))
	}

	// Gateway row (virtual index 5)
	gwLabel := m.srvFormGwLabel()
	if m.formFocusIdx == 5 && !m.srvFormGwPickerOpen {
		focused := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
		sb.WriteString(fmt.Sprintf("  %-12s  %s%s\n", "Gateway",
			focused.Render(gwLabel),
			m.dimStyle.Render("  [Enter] open picker")))
	} else {
		sb.WriteString(fmt.Sprintf("  %-12s  %s\n", "Gateway", m.dimStyle.Render(gwLabel)))
	}
	if m.srvFormGwPickerOpen {
		sb.WriteString(m.renderGwPicker())
	}

	// Locale (formFields[5], Tab-index 6)
	sb.WriteString(fmt.Sprintf("  %-12s  %s\n", "Locale", m.formFields[5].View()))

	sb.WriteString("\n")
	if m.srvFormGwPickerOpen {
		sb.WriteString(m.dimStyle.Render("  [↑↓] navigate  [type] search  [Enter] select  [Esc] close picker"))
	} else {
		sb.WriteString(m.dimStyle.Render("  [Tab] next  [Shift+Tab] prev  [Enter] save  [Esc] cancel"))
	}
	if m.statusMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(m.dimStyle.Render("  " + m.statusMsg))
	}
	return sb.String()
}

func (m Model) srvFormGwLabel() string {
	if m.srvFormSelectedGwID != nil {
		for _, gw := range m.gateways {
			if gw.ID == *m.srvFormSelectedGwID {
				return "[gw] " + gw.Name
			}
		}
	}
	if m.srvFormSelectedSrvGwID != nil {
		for _, s := range m.servers {
			if s.ID == *m.srvFormSelectedSrvGwID {
				return fmt.Sprintf("[srv] %s@%s", s.User, s.Host)
			}
		}
	}
	return "(none)"
}

func (m Model) renderGwPicker() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  Search: %s\n", m.srvFormGwSearch.View()))
	sb.WriteString("\n")

	entries := m.gwPickerEntries()
	viewport := 8
	total := len(entries)
	viewStart, viewEnd := m.viewWindow(m.srvFormGwPickerCursor, total, viewport)

	for i := viewStart; i < viewEnd; i++ {
		e := entries[i]
		if i == m.srvFormGwPickerCursor {
			sb.WriteString(m.selectedStyle.Render("  > " + e.label))
		} else {
			sb.WriteString(m.normalStyle.Render("    " + e.label))
		}
		sb.WriteString("\n")
	}
	if total > viewport {
		sb.WriteString(m.dimStyle.Render(fmt.Sprintf("    %d/%d", m.srvFormGwPickerCursor+1, total)))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m Model) renderConfirmDelete() string {
	if m.deleteTarget == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Delete %s@%s?\n\n", m.deleteTarget.User, m.deleteTarget.Host))
	sb.WriteString("[y] confirm  [n/Esc] cancel")
	return m.confirmStyle.Render(sb.String())
}

// ── gateway list/form ─────────────────────────────────────────────────────────

func (m Model) renderGatewayList() string {
	var sb strings.Builder
	sb.WriteString(m.titleStyle.Render("alogin — /gateway"))
	sb.WriteString("\n")
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	l1, l2 := pageDesc("gateway")
	sb.WriteString(descStyle.Render("  " + l1))
	sb.WriteString("\n")
	sb.WriteString(descStyle.Render("  " + l2))
	sb.WriteString("\n\n")

	// Fixed lines: title(1) desc(2) blank(1) hint(1) blank(1) = 6
	viewport := m.visibleRows(6)
	total := len(m.gateways)
	viewStart, viewEnd := m.viewWindow(m.gatewayCursor, total, viewport)

	if total == 0 {
		sb.WriteString(m.dimStyle.Render("  (no gateways defined)"))
	}
	for i := viewStart; i < viewEnd; i++ {
		gw := m.gateways[i]
		line := fmt.Sprintf("%-20s  %s", gw.Name, m.hopsSummary(gw))
		if i == m.gatewayCursor {
			sb.WriteString(m.selectedStyle.Render("> " + line))
		} else {
			sb.WriteString(m.normalStyle.Render("  " + line))
		}
		sb.WriteString("\n")
	}
	if total > viewport {
		sb.WriteString(m.dimStyle.Render(fmt.Sprintf("  %d/%d", m.gatewayCursor+1, total)))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(m.dimStyle.Render("[↑↓] navigate  [a] add  [e] edit  [d] delete  [Esc] back"))
	if m.statusMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(m.dimStyle.Render("  " + m.statusMsg))
	}
	return sb.String()
}

func (m Model) hopsSummary(gw *model.GatewayRoute) string {
	if len(gw.Hops) == 0 {
		return "(no hops)"
	}
	parts := make([]string, len(gw.Hops))
	for i, h := range gw.Hops {
		label := fmt.Sprintf("#%d", h.ServerID)
		for _, s := range m.servers {
			if s.ID == h.ServerID {
				label = fmt.Sprintf("%s@%s", s.User, s.Host)
				break
			}
		}
		parts[i] = label
	}
	return strings.Join(parts, " → ")
}

func (m Model) renderGatewayForm() string {
	var sb strings.Builder
	title := "Add Gateway"
	if m.gwFormMode == fmEdit && m.gwFormTarget != nil {
		title = fmt.Sprintf("Edit Gateway: %s", m.gwFormTarget.Name)
	}
	sb.WriteString(m.titleStyle.Render("alogin — " + title))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("  %-10s  %s\n", "Name", m.gwFormName.View()))
	sb.WriteString("\n")
	sb.WriteString(m.dimStyle.Render("  Hops (in order):"))
	sb.WriteString("\n")

	if len(m.gwFormHops) == 0 {
		sb.WriteString(m.dimStyle.Render("    (none)"))
		sb.WriteString("\n")
	}
	for i, sid := range m.gwFormHops {
		label := serverLabel(m.servers, sid)
		line := fmt.Sprintf("  %d. %s", i+1, label)
		if i == m.gwFormHopCursor && !m.gwFormName.Focused() {
			sb.WriteString(m.selectedStyle.Render("> " + strings.TrimSpace(line)))
		} else {
			sb.WriteString(m.normalStyle.Render("  " + strings.TrimSpace(line)))
		}
		sb.WriteString("\n")
	}

	if m.gwFormPickerOpen {
		sb.WriteString(m.renderServerPicker(m.gwFormPickerCursor))
	}

	sb.WriteString("\n")
	if m.gwFormName.Focused() {
		sb.WriteString(m.dimStyle.Render("  [Tab] hop list  [Ctrl+S] save  [Esc] cancel"))
	} else if m.gwFormPickerOpen {
		sb.WriteString(m.dimStyle.Render("  [↑↓] select  [Enter] add  [Esc] cancel picker"))
	} else {
		sb.WriteString(m.dimStyle.Render("  [↑↓] navigate  [a] add hop  [x] remove  [u] up  [m] down  [Tab] name  [Ctrl+S] save  [Esc] cancel"))
	}
	if m.statusMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(m.dimStyle.Render("  " + m.statusMsg))
	}
	return sb.String()
}

// ── cluster list/form ─────────────────────────────────────────────────────────

func (m Model) renderClusterList() string {
	var sb strings.Builder
	sb.WriteString(m.titleStyle.Render("alogin — /cluster"))
	sb.WriteString("\n")
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	l1, l2 := pageDesc("cluster")
	sb.WriteString(descStyle.Render("  " + l1))
	sb.WriteString("\n")
	sb.WriteString(descStyle.Render("  " + l2))
	sb.WriteString("\n\n")

	viewport := m.visibleRows(6)
	total := len(m.clusters)
	viewStart, viewEnd := m.viewWindow(m.clCursor, total, viewport)

	if total == 0 {
		sb.WriteString(m.dimStyle.Render("  (no clusters defined)"))
	}
	for i := viewStart; i < viewEnd; i++ {
		cl := m.clusters[i]
		line := fmt.Sprintf("%-20s  %d members", cl.Name, len(cl.Members))
		if i == m.clCursor {
			sb.WriteString(m.selectedStyle.Render("> " + line))
		} else {
			sb.WriteString(m.normalStyle.Render("  " + line))
		}
		sb.WriteString("\n")
	}
	if total > viewport {
		sb.WriteString(m.dimStyle.Render(fmt.Sprintf("  %d/%d", m.clCursor+1, total)))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(m.dimStyle.Render("[↑↓] navigate  [a] add  [e] edit  [d] delete  [Esc] back"))
	if m.statusMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(m.dimStyle.Render("  " + m.statusMsg))
	}
	return sb.String()
}

func (m Model) renderClusterForm() string {
	var sb strings.Builder
	title := "Add Cluster"
	if m.clFormMode == fmEdit && m.clFormTarget != nil {
		title = fmt.Sprintf("Edit Cluster: %s", m.clFormTarget.Name)
	}
	sb.WriteString(m.titleStyle.Render("alogin — " + title))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("  %-10s  %s\n", "Name", m.clFormName.View()))
	sb.WriteString("\n")
	sb.WriteString(m.dimStyle.Render("  Members (in order):"))
	sb.WriteString("\n")

	if len(m.clFormMembers) == 0 {
		sb.WriteString(m.dimStyle.Render("    (none)"))
		sb.WriteString("\n")
	}
	for i, mem := range m.clFormMembers {
		srv := serverByID(m.servers, mem.serverID)
		user := mem.user
		if user == "" && srv != nil {
			user = srv.User
		}
		host := fmt.Sprintf("#%d", mem.serverID)
		if srv != nil {
			host = srv.Host
		}
		userTag := ""
		if mem.user != "" {
			userTag = fmt.Sprintf(" [u:%s]", mem.user)
		}
		line := fmt.Sprintf("  %d. %s@%s%s", i+1, user, host, userTag)
		if i == m.clFormMemberCursor && !m.clFormName.Focused() {
			sb.WriteString(m.selectedStyle.Render("> " + strings.TrimSpace(line)))
		} else {
			sb.WriteString(m.normalStyle.Render("  " + strings.TrimSpace(line)))
		}
		sb.WriteString("\n")
	}

	if m.clFormUserEditOpen {
		sb.WriteString("\n")
		sb.WriteString(m.dimStyle.Render("  User override:"))
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("  %s\n", m.clFormUserInput.View()))
		sb.WriteString(m.dimStyle.Render("  [Enter] confirm  [Esc] cancel"))
		sb.WriteString("\n")
	} else if m.clFormPickerOpen {
		sb.WriteString(m.renderServerPicker(m.clFormPickerCursor))
	}

	if !m.clFormUserEditOpen && !m.clFormPickerOpen {
		sb.WriteString("\n")
		if m.clFormName.Focused() {
			sb.WriteString(m.dimStyle.Render("  [Tab] member list  [Ctrl+S] save  [Esc] cancel"))
		} else {
			sb.WriteString(m.dimStyle.Render("  [↑↓] navigate  [a] add  [x] remove  [u] move up  [m] move down  [r] set user  [Tab] name  [Ctrl+S] save  [Esc] cancel"))
		}
	}
	if m.statusMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(m.dimStyle.Render("  " + m.statusMsg))
	}
	return sb.String()
}

// ── local hosts list/form ─────────────────────────────────────────────────────

func (m Model) renderHostList() string {
	var sb strings.Builder
	sb.WriteString(m.titleStyle.Render("alogin — /hosts"))
	sb.WriteString("\n")
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	l1, l2 := pageDesc("hosts")
	sb.WriteString(descStyle.Render("  " + l1))
	sb.WriteString("\n")
	sb.WriteString(descStyle.Render("  " + l2))
	sb.WriteString("\n\n")

	viewport := m.visibleRows(6)
	total := len(m.localHosts)
	viewStart, viewEnd := m.viewWindow(m.hostCursor, total, viewport)

	if total == 0 {
		sb.WriteString(m.dimStyle.Render("  (no local host mappings defined)"))
	}
	for i := viewStart; i < viewEnd; i++ {
		h := m.localHosts[i]
		desc := h.Description
		if desc == "" {
			desc = "-"
		}
		line := fmt.Sprintf("%-30s  %-20s  %s", h.Hostname, h.IP, desc)
		if i == m.hostCursor {
			sb.WriteString(m.selectedStyle.Render("> " + line))
		} else {
			sb.WriteString(m.normalStyle.Render("  " + line))
		}
		sb.WriteString("\n")
	}
	if total > viewport {
		sb.WriteString(m.dimStyle.Render(fmt.Sprintf("  %d/%d", m.hostCursor+1, total)))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(m.dimStyle.Render("[↑↓] navigate  [a] add  [e] edit  [d] delete  [Esc] back"))
	if m.statusMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(m.dimStyle.Render("  " + m.statusMsg))
	}
	return sb.String()
}

func (m Model) renderHostForm() string {
	var sb strings.Builder
	title := "Add Local Host"
	if m.hostFormMode == fmEdit && m.hostFormTarget != nil {
		title = fmt.Sprintf("Edit Local Host: %s", m.hostFormTarget.Hostname)
	}
	sb.WriteString(m.titleStyle.Render("alogin — " + title))
	sb.WriteString("\n\n")

	labels := []string{"Hostname", "IP", "Description"}
	for i, field := range m.hostFormFields {
		label := labels[i]
		if m.hostFormMode == fmEdit && i == 0 {
			label += " (locked)"
		}
		sb.WriteString(fmt.Sprintf("  %-14s  %s\n", label, field.View()))
	}
	sb.WriteString("\n")
	sb.WriteString(m.dimStyle.Render("  [Tab] next  [Shift+Tab] prev  [Ctrl+S] save  [Enter] next/save  [Esc] cancel"))
	if m.statusMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(m.dimStyle.Render("  " + m.statusMsg))
	}
	return sb.String()
}

// ── tunnel list/form ──────────────────────────────────────────────────────────

func (m Model) renderTunnelList() string {
	var sb strings.Builder
	sb.WriteString(m.titleStyle.Render("alogin — /tunnel"))
	sb.WriteString("\n")
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	l1, l2 := pageDesc("tunnel")
	sb.WriteString(descStyle.Render("  " + l1))
	sb.WriteString("\n")
	sb.WriteString(descStyle.Render("  " + l2))
	sb.WriteString("\n\n")

	viewport := m.visibleRows(7)
	total := len(m.tunnels)
	viewStart, viewEnd := m.viewWindow(m.tunnelCursor, total, viewport)

	if total == 0 {
		sb.WriteString(m.dimStyle.Render("  (no tunnels configured)"))
	}
	for i := viewStart; i < viewEnd; i++ {
		t := m.tunnels[i]
		srv := serverByID(m.servers, t.ServerID)
		srvLabel := fmt.Sprintf("id=%d", t.ServerID)
		if srv != nil {
			srvLabel = srv.Host
		}
		running := m.tnStatuses[t.ID]
		statusTag := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("[stopped]")
		if running {
			statusTag = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("[running]")
		}
		autoGWTag := ""
		if t.AutoGW {
			autoGWTag = " gw"
		}
		line := fmt.Sprintf("%-18s  %-16s  %s  %s:%d → %s:%d%s",
			t.Name, srvLabel, string(t.Direction),
			t.LocalHost, t.LocalPort, t.RemoteHost, t.RemotePort,
			autoGWTag,
		)
		if i == m.tunnelCursor {
			sb.WriteString(m.selectedStyle.Render("> "+line) + "  " + statusTag)
		} else {
			sb.WriteString(m.normalStyle.Render("  "+line) + "  " + statusTag)
		}
		sb.WriteString("\n")
	}
	if total > viewport {
		sb.WriteString(m.dimStyle.Render(fmt.Sprintf("  %d/%d", m.tunnelCursor+1, total)))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(m.dimStyle.Render("[↑↓] navigate  [s] start  [x] stop  [a] add  [e] edit  [d] delete  [Esc] back"))
	if m.statusMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(m.dimStyle.Render("  " + m.statusMsg))
	}
	return sb.String()
}

func (m Model) renderTunnelForm() string {
	var sb strings.Builder
	title := "Add Tunnel"
	if m.tnFormMode == fmEdit && m.tnFormTarget != nil {
		title = fmt.Sprintf("Edit Tunnel: %s", m.tnFormTarget.Name)
	}
	sb.WriteString(m.titleStyle.Render("alogin — " + title))
	sb.WriteString("\n\n")

	// Name field (tab stop 0)
	sb.WriteString(fmt.Sprintf("  %-16s  %s\n", "Name", m.tnFormFields[0].View()))

	// Server picker row (tab stop 1)
	srvLabel := "(none — press Enter to pick)"
	if m.tnFormServerID != 0 {
		if srv := serverByID(m.servers, m.tnFormServerID); srv != nil {
			srvLabel = fmt.Sprintf("%s@%s", srv.User, srv.Host)
		} else {
			srvLabel = fmt.Sprintf("id=%d", m.tnFormServerID)
		}
	}
	srvRow := fmt.Sprintf("  %-16s  %s", "Server", srvLabel)
	if m.tnFormFocus == tnFormIdxServer {
		sb.WriteString(m.selectedStyle.Render("> " + strings.TrimPrefix(srvRow, "  ")))
	} else {
		sb.WriteString(m.dimStyle.Render(srvRow))
	}
	sb.WriteString("\n")

	// Server picker overlay
	if m.tnFormPickerOpen {
		sb.WriteString(m.renderServerPicker(m.tnFormPickerCursor))
	}

	// Remaining text fields: [1]=direction [2]=localHost [3]=localPort [4]=remoteHost [5]=remotePort
	labels := []string{"Direction (L/R)", "Local Host", "Local Port", "Remote Host", "Remote Port"}
	for i, field := range m.tnFormFields[1:] {
		sb.WriteString(fmt.Sprintf("  %-16s  %s\n", labels[i], field.View()))
	}

	// AutoGW toggle (tab stop index 7)
	autoGWDisplay := "[ ] Auto-GW"
	if m.tnFormAutoGW {
		autoGWDisplay = "[x] Auto-GW"
	}
	if m.tnFormFocus == tnFormIdxAutoGW {
		sb.WriteString(m.selectedStyle.Render("> " + autoGWDisplay))
	} else {
		sb.WriteString(m.dimStyle.Render("  " + autoGWDisplay))
	}
	sb.WriteString("\n\n")
	sb.WriteString(m.dimStyle.Render("  [Tab] next  [Shift+Tab] prev  [Space] toggle AutoGW  [Ctrl+S] save  [Esc] cancel"))
	if m.statusMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(m.dimStyle.Render("  " + m.statusMsg))
	}
	return sb.String()
}

// ── shared helpers ────────────────────────────────────────────────────────────

func (m Model) renderServerPicker(cursor int) string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(m.dimStyle.Render("  Pick server:"))
	sb.WriteString("\n")
	for i, s := range m.servers {
		line := fmt.Sprintf("%s@%s", s.User, s.Host)
		if i == cursor {
			sb.WriteString(m.selectedStyle.Render("  > " + line))
		} else {
			sb.WriteString(m.normalStyle.Render("    " + line))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func serverLabel(servers []*model.Server, id int64) string {
	for _, s := range servers {
		if s.ID == id {
			return fmt.Sprintf("%s@%s", s.User, s.Host)
		}
	}
	return fmt.Sprintf("#%d", id)
}

func serverByID(servers []*model.Server, id int64) *model.Server {
	for _, s := range servers {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// visibleRows returns how many list rows fit in the terminal given the number
// of lines consumed by fixed chrome (title, input bar, hints, etc.).
// Falls back to 20 when terminal size is unknown.
func (m Model) visibleRows(fixedLines int) int {
	if m.termHeight <= 0 {
		return 20
	}
	n := m.termHeight - fixedLines
	if n < 3 {
		n = 3
	}
	return n
}

// viewWindow returns the [start, end) slice indices that keep cursor visible
// inside a viewport of the given size.
func (m Model) viewWindow(cursor, total, viewport int) (start, end int) {
	if total == 0 {
		return 0, 0
	}
	start = cursor - viewport + 1
	if start < 0 {
		start = 0
	}
	end = start + viewport
	if end > total {
		end = total
		start = end - viewport
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

// ── app-server list / form ────────────────────────────────────────────────────

func (m Model) renderAppServerList() string {
	var sb strings.Builder
	sb.WriteString(m.titleStyle.Render("alogin — /app-server"))
	sb.WriteString("\n")
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	sb.WriteString(descStyle.Render("  Named server+plugin bindings. Enter to connect, a add, e edit, d delete."))
	sb.WriteString("\n\n")

	viewport := m.visibleRows(7)
	total := len(m.appServers)
	viewStart, viewEnd := m.viewWindow(m.appServerCursor, total, viewport)

	if total == 0 {
		sb.WriteString(m.dimStyle.Render("  (no app-server bindings configured)"))
	}
	for i := viewStart; i < viewEnd; i++ {
		as := m.appServers[i]
		srvLabel := fmt.Sprintf("id=%d", as.ServerID)
		if srv := serverByID(m.servers, as.ServerID); srv != nil {
			srvLabel = srv.Host
		}
		gwTag := ""
		if as.AutoGW {
			gwTag = " [gw]"
		}
		desc := ""
		if as.Description != "" {
			desc = "  " + as.Description
		}
		line := fmt.Sprintf("%-20s  %-16s  %-12s%s%s",
			as.Name, srvLabel, as.PluginName, gwTag, desc)
		if i == m.appServerCursor {
			sb.WriteString(m.selectedStyle.Render("> " + line))
		} else {
			sb.WriteString(m.normalStyle.Render("  " + line))
		}
		sb.WriteString("\n")
	}
	if total > viewport {
		sb.WriteString(m.dimStyle.Render(fmt.Sprintf("  %d/%d", m.appServerCursor+1, total)))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(m.dimStyle.Render("[↑↓] navigate  [Enter] connect  [a] add  [e] edit  [d] delete  [Esc] back"))
	if m.statusMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(m.dimStyle.Render("  " + m.statusMsg))
	}
	return sb.String()
}

func (m Model) renderAppServerForm() string {
	var sb strings.Builder
	title := "Add App-Server"
	if m.asFormMode == fmEdit && m.asFormTarget != nil {
		title = fmt.Sprintf("Edit App-Server: %s", m.asFormTarget.Name)
	}
	sb.WriteString(m.titleStyle.Render("alogin — " + title))
	sb.WriteString("\n\n")

	// Text fields: name, plugin, description
	labels := []string{"Name", "Plugin", "Description"}
	for i, field := range m.asFormFields {
		sb.WriteString(fmt.Sprintf("  %-16s  %s\n", labels[i], field.View()))
	}

	// Server picker row (tab stop index 3)
	srvLabel := "(none — press Enter to pick)"
	if m.asFormServerID != 0 {
		if srv := serverByID(m.servers, m.asFormServerID); srv != nil {
			srvLabel = fmt.Sprintf("%s@%s", srv.User, srv.Host)
		} else {
			srvLabel = fmt.Sprintf("id=%d", m.asFormServerID)
		}
	}
	srvRow := fmt.Sprintf("  %-16s  %s", "Server", srvLabel)
	if m.asFormFocus == asFormIdxServer {
		sb.WriteString(m.selectedStyle.Render("> " + strings.TrimPrefix(srvRow, "  ")))
	} else {
		sb.WriteString(m.dimStyle.Render(srvRow))
	}
	sb.WriteString("\n")

	// Server picker overlay
	if m.asFormPickerOpen {
		sb.WriteString(m.renderServerPicker(m.asFormPickerCursor))
	}

	// AutoGW toggle (tab stop index 4)
	autoGWDisplay := "[ ] Auto-GW"
	if m.asFormAutoGW {
		autoGWDisplay = "[x] Auto-GW"
	}
	if m.asFormFocus == asFormIdxAutoGW {
		sb.WriteString(m.selectedStyle.Render("> " + autoGWDisplay))
	} else {
		sb.WriteString(m.dimStyle.Render("  " + autoGWDisplay))
	}
	sb.WriteString("\n\n")
	sb.WriteString(m.dimStyle.Render("  [Tab] next  [Shift+Tab] prev  [Enter] toggle AutoGW  [Ctrl+S] save  [Esc] cancel"))
	if m.statusMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(m.dimStyle.Render("  " + m.statusMsg))
	}
	return sb.String()
}

// ── plugin picker ─────────────────────────────────────────────────────────────

func (m Model) renderPluginPicker() string {
	var sb strings.Builder

	title := m.titleStyle.Render("  Select Application Plugin  ")
	sb.WriteString(title + "\n\n")

	if len(m.plugins) == 0 {
		sb.WriteString(m.dimStyle.Render("No plugins found in plugins directory.") + "\n")
		sb.WriteString(m.dimStyle.Render("Add YAML files to: "+m.pluginDir) + "\n\n")
	} else {
		for i, name := range m.plugins {
			if i == m.pluginCursor {
				sb.WriteString(m.selectedStyle.Render("  "+name) + "\n")
			} else {
				sb.WriteString(m.normalStyle.Render("  "+name) + "\n")
			}
		}
		sb.WriteString("\n")
	}

	hint := m.dimStyle.Render("↑/↓ navigate  enter select  esc cancel")
	sb.WriteString(hint + "\n")
	return sb.String()
}

// ── plugin list ───────────────────────────────────────────────────────────────

func (m Model) renderPluginList() string {
	var sb strings.Builder

	title := m.titleStyle.Render("  Application Plugins  ")
	sb.WriteString(title + "\n\n")

	if len(m.pluginList) == 0 {
		sb.WriteString(m.dimStyle.Render("No plugins found.") + "\n")
		sb.WriteString(m.dimStyle.Render("Add YAML files to: "+m.pluginDir) + "\n\n")
	} else {
		colName := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
		colDim := m.dimStyle

		for i, p := range m.pluginList {
			strategies := strings.Join(p.Runtime.Strategies, ",")
			line := fmt.Sprintf("  %-20s  v%-6s  %-8s  %s",
				p.Name, p.Version, string(p.Auth.Provider), strategies)
			if i == m.pluginListCursor {
				sb.WriteString(m.selectedStyle.Render(line) + "\n")
			} else {
				sb.WriteString(colName.Render("  "+p.Name) +
					colDim.Render(fmt.Sprintf("  v%-6s  %-8s  %s", p.Version, string(p.Auth.Provider), strategies)) + "\n")
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString(m.dimStyle.Render("↑/↓ navigate  [Enter] detail  esc back"))
	return sb.String()
}

// ── plugin detail ─────────────────────────────────────────────────────────────

func (m Model) renderPluginDetail() string {
	p := m.pluginDetail
	if p == nil {
		return m.dimStyle.Render("No plugin selected.")
	}

	var sb strings.Builder

	accent := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
	dim := m.dimStyle

	// ── header ──
	sb.WriteString(accent.Render("  "+p.Name) +
		dim.Render(fmt.Sprintf("  v%s", p.Version)) + "\n\n")

	// ── auth ──
	sb.WriteString(label.Render("  Auth") + dim.Render(fmt.Sprintf("  provider: %s", p.Auth.Provider)) + "\n")
	for _, mv := range p.Auth.Mapping {
		sb.WriteString(fmt.Sprintf("    %s  expose:%-8s  path: %s\n",
			accent.Render(mv.Var), string(mv.Expose), mv.Path))
		if mv.Automation != nil {
			sb.WriteString(dim.Render(fmt.Sprintf("      expect: %q  send_newline: %v\n",
				mv.Automation.Expect, mv.Automation.SendNewline)))
		}
	}
	sb.WriteString("\n")

	// ── runtime ──
	sb.WriteString(label.Render("  Runtime") +
		dim.Render(fmt.Sprintf("  strategies: %s", strings.Join(p.Runtime.Strategies, " → "))) + "\n")
	if n := p.Runtime.Environments.Native; n != nil {
		sb.WriteString(fmt.Sprintf("    native   %s %s\n",
			accent.Render(n.Command), dim.Render(strings.Join(n.Args, " "))))
	}
	if d := p.Runtime.Environments.Docker; d != nil {
		sb.WriteString(fmt.Sprintf("    docker   match:%s  %s %s\n",
			d.ContainerMatch, accent.Render(d.Command), dim.Render(strings.Join(d.Args, " "))))
	}
	sb.WriteString("\n")

	// ── status / hint ──
	if m.statusMsg != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("  "+m.statusMsg) + "\n")
	}
	sb.WriteString(dim.Render("  [e] edit in $EDITOR  esc back"))
	return sb.String()
}

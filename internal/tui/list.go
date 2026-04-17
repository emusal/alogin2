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
	case stateProfileList:
		return m.renderProfileList()
	case stateProfileForm:
		return m.renderProfileForm()
	}
	return m.renderMainList()
}

// ── layout engine ─────────────────────────────────────────────────────────────

// w returns the usable terminal width (minimum 40).
func (m Model) w() int {
	if m.termWidth > 0 {
		return m.termWidth
	}
	return 80
}

// h returns the terminal height (minimum 10).
func (m Model) h() int {
	if m.termHeight > 0 {
		return m.termHeight
	}
	return 24
}

// renderScreen assembles a full-screen layout:
//
//	header bar  (1 line)
//	body        (h-3 lines, padded to fill)
//	status bar  (1 line)
//	hint bar    (1 line)
func (m Model) renderScreen(title, subtitle, body, hint string) string {
	w := m.w()
	totalH := m.h()

	// ── header bar ──
	accent := lipgloss.Color("212")
	headerBg := lipgloss.Color("235")

	logoStyle := lipgloss.NewStyle().
		Bold(true).Foreground(accent).Background(headerBg)
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).Background(headerBg)
	headerFill := lipgloss.NewStyle().Background(headerBg)

	left := logoStyle.Render(" alogin ") + titleStyle.Render("· "+subtitle)

	// right side: active profile badge + optional title
	right := ""
	for _, p := range m.profiles {
		if p.IsActive {
			right = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).Background(headerBg).
				Render("⬡ "+p.Name+" ") + right
			break
		}
	}
	if title != "" {
		right = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).Background(headerBg).
			Render(title+" ") + right
	}
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	padW := w - leftW - rightW
	if padW < 0 {
		padW = 0
	}
	header := left + headerFill.Render(strings.Repeat(" ", padW)) + right

	// ── status bar ──
	statusBg := lipgloss.Color("234")
	statusStyle := lipgloss.NewStyle().Background(statusBg).Foreground(lipgloss.Color("240"))
	statusContent := " "
	if m.statusMsg != "" {
		// colour: green for ok, red for Error:
		if strings.HasPrefix(m.statusMsg, "Error:") {
			statusContent = lipgloss.NewStyle().
				Background(statusBg).Foreground(lipgloss.Color("196")).
				Render(" " + m.statusMsg)
		} else {
			statusContent = lipgloss.NewStyle().
				Background(statusBg).Foreground(lipgloss.Color("82")).
				Render(" " + m.statusMsg)
		}
	}
	statusBar := statusStyle.Width(w).Render(statusContent)

	// ── hint bar ──
	hintBg := lipgloss.Color("237")
	hintBar := lipgloss.NewStyle().
		Background(hintBg).Foreground(lipgloss.Color("244")).
		Width(w).
		Render(" " + hint)

	// ── body: pad empty lines to fill remaining height ──
	// header(1) + statusBar(1) + hintBar(1) = 3 fixed lines
	bodyH := totalH - 3
	if bodyH < 1 {
		bodyH = 1
	}
	bodyLines := strings.Split(body, "\n")
	// Ensure we don't exceed bodyH (truncate if form/content is long)
	if len(bodyLines) > bodyH {
		bodyLines = bodyLines[:bodyH]
	}
	// Pad remaining lines
	for len(bodyLines) < bodyH {
		bodyLines = append(bodyLines, "")
	}
	paddedBody := strings.Join(bodyLines, "\n")

	return strings.Join([]string{header, paddedBody, statusBar, hintBar}, "\n")
}

// bodyHeight returns how many lines the body section has.
func (m Model) bodyHeight() int {
	h := m.h() - 3
	if h < 3 {
		h = 3
	}
	return h
}

// ── welcome screen ────────────────────────────────────────────────────────────

func (m Model) renderWelcome() string {
	w := m.w()
	totalH := m.h()

	accent := lipgloss.Color("212")
	purple := lipgloss.Color("99")

	// Header bar
	headerBg := lipgloss.Color("235")
	header := lipgloss.NewStyle().
		Background(headerBg).Foreground(accent).Bold(true).
		Width(w).Render(" alogin  ·  SSH Connection Manager")

	// Center box
	boxW := min(w-8, 62)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(purple).
		Padding(1, 3).
		Width(boxW)

	logoLine := lipgloss.NewStyle().Bold(true).Foreground(accent).Render("alogin") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  ·  SSH Connection Manager")
	versionLine := lipgloss.NewStyle().Foreground(purple).Render("v" + m.version)

	statStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	stats := fmt.Sprintf("%d servers  ·  %d gateways  ·  %d clusters",
		len(m.servers), m.gwCount, m.clCount)

	boxContent := logoLine + "\n" + versionLine + "\n\n" + statStyle.Render(stats)
	box := boxStyle.Render(boxContent)

	// Input hint box
	inputW := boxW
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(0, 2).
		Width(inputW)

	promptStr := lipgloss.NewStyle().Bold(true).Foreground(accent).Render(">")
	phStr := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Search hosts or type / for commands...")
	inputBox := inputStyle.Render(promptStr + " " + phStr + "▊")

	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	hints := hintStyle.Render("/server  ·  /gateway  ·  /profile  ·  /cluster  ·  /tunnel  ·  a add  ·  q quit")

	// Vertical centering
	contentLines := strings.Split(box+"\n\n"+inputBox+"\n\n"+hints, "\n")
	bodyH := totalH - 2 // header(1) + hint(1)
	topPad := (bodyH - len(contentLines)) / 2
	if topPad < 0 {
		topPad = 0
	}

	var sb strings.Builder
	sb.WriteString(header + "\n")
	for i := 0; i < topPad; i++ {
		sb.WriteString("\n")
	}
	// Center box horizontally
	padLeft := (w - lipgloss.Width(box)) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	leftPad := strings.Repeat(" ", padLeft)
	for _, line := range strings.Split(box, "\n") {
		sb.WriteString(leftPad + line + "\n")
	}
	sb.WriteString("\n")
	for _, line := range strings.Split(inputBox, "\n") {
		sb.WriteString(leftPad + line + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(leftPad + hints + "\n")

	// Fill to bottom - 1 (leave room for hint bar)
	current := topPad + len(strings.Split(box, "\n")) + 1 + len(strings.Split(inputBox, "\n")) + 1 + 1
	for i := current; i < bodyH-1; i++ {
		sb.WriteString("\n")
	}

	hintBg := lipgloss.Color("237")
	hintBar := lipgloss.NewStyle().
		Background(hintBg).Foreground(lipgloss.Color("244")).
		Width(w).
		Render(" q quit  ·  Enter open server list")
	sb.WriteString(hintBar)

	return sb.String()
}

// ── main list (server list) ───────────────────────────────────────────────────

func (m Model) renderMainList() string {
	w := m.w()
	l1, _ := pageDesc("server")

	// Input bar
	promptStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	var inputContent string
	if m.query == "" {
		ph := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Search hosts or type / for commands...")
		inputContent = promptStyle.Render(">") + " " + ph + "▊"
	} else {
		inputContent = promptStyle.Render(">") + " " + m.query + "▊"
	}
	inputBar := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color("238")).
		Width(w - 2).
		Padding(0, 1).
		Render(inputContent)

	// Slash-command palette
	if strings.HasPrefix(m.query, "/") {
		body := "\n" + inputBar + "\n\n" + m.renderCommandPalette()
		return m.renderScreen("", l1, body, "↑↓ navigate  Enter open  Tab complete  Esc cancel")
	}

	// Detail overlay
	if m.state == stateDetail && len(m.filtered) > 0 {
		body := "\n" + inputBar + "\n\n" + m.renderDetail(m.filtered[m.cursor])
		hint := "Enter connect  e edit  d delete  Tab/Esc back"
		return m.renderScreen("", l1, body, hint)
	}

	// List: bodyH minus input(3) minus blank(1) minus col-header(1) = 5 overhead
	listH := m.bodyHeight() - 6
	if listH < 1 {
		listH = 1
	}
	total := len(m.filtered)
	viewStart, viewEnd := m.viewWindow(m.cursor, total, listH)

	// Column header
	colW := m.serverColWidths()
	colHeader := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Background(lipgloss.Color("234")).
		Width(w).
		Render(fmt.Sprintf("  %-*s  %-*s  %-*s  %s",
			colW[0], "HOST",
			colW[1], "USER",
			colW[2], "PROTO",
			"PORT"))

	var listSb strings.Builder
	if total == 0 {
		listSb.WriteString(m.dimStyle.Render("  (no results)") + "\n")
	}
	for i := viewStart; i < viewEnd; i++ {
		s := m.filtered[i]
		portStr := "—"
		if s.Port > 0 {
			portStr = fmt.Sprintf("%d", s.Port)
		}
		line := fmt.Sprintf("%-*s  %-*s  %-*s  %s",
			colW[0], s.Host,
			colW[1], s.User,
			colW[2], string(s.Protocol),
			portStr)
		if i == m.cursor {
			listSb.WriteString(m.selectedStyle.Width(w).Render("> " + line) + "\n")
		} else {
			listSb.WriteString(m.normalStyle.Render("  " + line) + "\n")
		}
	}
	// Pagination indicator
	if total > listH {
		listSb.WriteString(m.dimStyle.Render(fmt.Sprintf("  %d/%d", m.cursor+1, total)) + "\n")
	}

	body := "\n" + inputBar + "\n" + colHeader + "\n" + listSb.String()

	hint := "↑↓ navigate  Enter connect  r via-gateway  Tab details  a add  e edit  d delete  / commands  q quit"
	if m.query != "" {
		hint = "↑↓ navigate  Enter connect  q clear  Esc clear"
	}
	return m.renderScreen("", l1, body, hint)
}

// serverColWidths returns [hostW, userW, protoW] based on terminal width.
func (m Model) serverColWidths() [3]int {
	w := m.w()
	// fixed: "  " prefix(2) + "  " sep(2)*3 + proto(6) + port(5) = ~19 overhead
	remaining := w - 19
	if remaining < 20 {
		remaining = 20
	}
	hostW := remaining * 45 / 100
	userW := remaining * 30 / 100
	protoW := 6
	return [3]int{hostW, userW, protoW}
}

func (m Model) renderCommandPalette() string {
	cmds := m.filteredCommands()
	var sb strings.Builder
	if len(cmds) == 0 {
		sb.WriteString(m.dimStyle.Render("  (no matching commands)") + "\n")
		return sb.String()
	}
	for i, c := range cmds {
		line := fmt.Sprintf("%-14s  %s", c.trigger, c.desc)
		if i == m.cmdCursor {
			sb.WriteString(m.selectedStyle.Render("> " + line) + "\n")
		} else {
			sb.WriteString(m.normalStyle.Render("  " + line) + "\n")
		}
	}
	return sb.String()
}

// ── server detail ─────────────────────────────────────────────────────────────

func (m Model) renderDetail(s *model.Server) string {
	accent := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	val := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))

	row := func(k, v string) string {
		return label.Render(fmt.Sprintf("  %-10s", k)) + val.Render(v) + "\n"
	}

	port := "default"
	if s.Port > 0 {
		port = fmt.Sprintf("%d", s.Port)
	}
	gwLabel := "—"
	if s.GatewayRouteID != nil {
		gwLabel = fmt.Sprintf("route#%d", *s.GatewayRouteID)
		for _, gw := range m.gateways {
			if gw.ID == *s.GatewayRouteID {
				gwLabel = gw.Name
				break
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(accent.Render("  "+s.User+"@"+s.Host) + "\n\n")
	sb.WriteString(row("Protocol", string(s.Protocol)))
	sb.WriteString(row("Port", port))
	if s.Locale != "" && s.Locale != "-" {
		sb.WriteString(row("Locale", s.Locale))
	}
	sb.WriteString(row("Gateway", gwLabel))
	return sb.String()
}

// ── server form ───────────────────────────────────────────────────────────────

func (m Model) renderServerForm() string {
	title := "Add Server"
	if m.formMode == fmEdit && m.formTarget != nil {
		title = "Edit  " + m.formTarget.User + "@" + m.formTarget.Host
	}

	textLabels := []string{"Protocol", "Host", "User", "Password", "Port"}
	var body strings.Builder
	body.WriteString("\n")
	for i, field := range m.formFields[:5] {
		lbl := textLabels[i]
		if m.formMode == fmEdit && i == 1 {
			lbl += " (locked)"
		}
		focused := m.formFocusIdx == i
		body.WriteString(m.formRow(lbl, field.View(), focused))
	}

	// Gateway row (virtual index 5)
	gwLabel := m.srvFormGwLabel()
	if m.formFocusIdx == 5 && !m.srvFormGwPickerOpen {
		body.WriteString(m.formRow("Gateway",
			lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true).Render(gwLabel)+
				m.dimStyle.Render("  [Enter] open picker"), true))
	} else {
		body.WriteString(m.formRow("Gateway", m.dimStyle.Render(gwLabel), false))
	}
	if m.srvFormGwPickerOpen {
		body.WriteString(m.renderGwPicker())
	}

	// Locale (index 6)
	body.WriteString(m.formRow("Locale", m.formFields[5].View(), m.formFocusIdx == 6))

	// Auth Method (index 7) — picker row
	authLabel := m.srvFormAuthMethod
	if m.formFocusIdx == 7 && !m.srvFormAuthPickerOpen {
		body.WriteString(m.formRow("Auth Method",
			lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true).Render(authLabel)+
				m.dimStyle.Render("  [Enter] change"), true))
	} else {
		body.WriteString(m.formRow("Auth Method", m.dimStyle.Render(authLabel), false))
	}
	if m.srvFormAuthPickerOpen {
		body.WriteString(m.renderAuthPicker())
	}

	// Identity File (index 8) — only shown when auth_method == "key"
	if m.srvFormAuthMethod == "key" {
		body.WriteString(m.formRow("Identity File", m.formFields[6].View(), m.formFocusIdx == 8))
	}

	hint := "Tab next  Shift+Tab prev  Enter save  Esc cancel"
	if m.srvFormGwPickerOpen || m.srvFormAuthPickerOpen {
		hint = "↑↓ navigate  Enter select  Esc close picker"
	}
	bodyStr := body.String()
	if m.formEscConfirm {
		bodyStr = m.renderEscConfirmOverlay(bodyStr)
		hint = "Tab toggle  Enter confirm  Esc back to form"
	}
	return m.renderScreen(title, "server registry", bodyStr, hint)
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

func (m Model) renderAuthPicker() string {
	var sb strings.Builder
	sb.WriteString("\n")
	for i, v := range authMethods {
		if i == m.srvFormAuthPickerCursor {
			sb.WriteString(m.selectedStyle.Render("  > " + v))
		} else {
			sb.WriteString(m.normalStyle.Render("    " + v))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m Model) renderConfirmDelete() string {
	if m.deleteTarget == nil {
		return ""
	}
	w := m.w()
	accent := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Padding(1, 3).
		Width(min(w-8, 50))

	content := accent.Render("Delete server?") + "\n\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("255")).
			Render(m.deleteTarget.User+"@"+m.deleteTarget.Host) + "\n\n" +
		m.dimStyle.Render("[y] confirm  [n / Esc] cancel")

	box := boxStyle.Render(content)
	padLeft := (w - lipgloss.Width(box)) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	body := "\n\n\n\n" + strings.Repeat(" ", padLeft) + box
	return m.renderScreen("Confirm Delete", "server registry", body, "y confirm  n / Esc cancel")
}

// ── gateway list/form ─────────────────────────────────────────────────────────

func (m Model) renderGatewayList() string {
	l1, _ := pageDesc("gateway")
	listH := m.bodyHeight() - 3
	if listH < 1 {
		listH = 1
	}
	w := m.w()

	colHeader := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).Background(lipgloss.Color("234")).
		Width(w).Render(fmt.Sprintf("  %-20s  %s", "NAME", "HOPS"))

	total := len(m.gateways)
	viewStart, viewEnd := m.viewWindow(m.gatewayCursor, total, listH)

	var listSb strings.Builder
	if total == 0 {
		listSb.WriteString(m.dimStyle.Render("  (no gateways defined)") + "\n")
	}
	for i := viewStart; i < viewEnd; i++ {
		gw := m.gateways[i]
		hops := m.hopsSummary(gw)
		nameW := 20
		line := fmt.Sprintf("%-*s  %s", nameW, gw.Name, hops)
		if i == m.gatewayCursor {
			listSb.WriteString(m.selectedStyle.Width(w).Render("> "+line) + "\n")
		} else {
			listSb.WriteString(m.normalStyle.Render("  "+line) + "\n")
		}
	}
	if total > listH {
		listSb.WriteString(m.dimStyle.Render(fmt.Sprintf("  %d/%d", m.gatewayCursor+1, total)) + "\n")
	}

	body := "\n" + colHeader + "\n" + listSb.String()
	return m.renderScreen("/gateway", l1, body, "↑↓ navigate  a add  e edit  d delete  Esc back")
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
	title := "Add Gateway"
	if m.gwFormMode == fmEdit && m.gwFormTarget != nil {
		title = "Edit  " + m.gwFormTarget.Name
	}

	var body strings.Builder
	body.WriteString("\n")
	body.WriteString(m.formRow("Name", m.gwFormName.View(), m.gwFormName.Focused()))
	body.WriteString("\n")
	body.WriteString(m.dimStyle.Render("  Hops (in order):") + "\n")

	if len(m.gwFormHops) == 0 {
		body.WriteString(m.dimStyle.Render("    (none — press 'a' to add)") + "\n")
	}
	for i, sid := range m.gwFormHops {
		lbl := serverLabel(m.servers, sid)
		line := fmt.Sprintf("  %d. %s", i+1, lbl)
		if i == m.gwFormHopCursor && !m.gwFormName.Focused() {
			body.WriteString(m.selectedStyle.Render("> "+strings.TrimSpace(line)) + "\n")
		} else {
			body.WriteString(m.normalStyle.Render("  "+strings.TrimSpace(line)) + "\n")
		}
	}

	if m.gwFormPickerOpen {
		body.WriteString(m.renderServerPicker(m.gwFormPickerCursor))
	}

	hint := "↑↓ navigate  a add hop  x remove  u up  m down  Tab name  Ctrl+S save  Esc cancel"
	if m.gwFormName.Focused() {
		hint = "Tab hop list  Ctrl+S save  Esc cancel"
	} else if m.gwFormPickerOpen {
		hint = "↑↓ select  Enter add  Esc cancel picker"
	}
	bodyStr := body.String()
	if m.formEscConfirm {
		bodyStr = m.renderEscConfirmOverlay(bodyStr)
		hint = "Tab toggle  Enter confirm  Esc back to form"
	}
	return m.renderScreen(title, "gateway", bodyStr, hint)
}

// ── cluster list/form ─────────────────────────────────────────────────────────

func (m Model) renderClusterList() string {
	l1, _ := pageDesc("cluster")
	listH := m.bodyHeight() - 3
	if listH < 1 {
		listH = 1
	}
	w := m.w()

	colHeader := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).Background(lipgloss.Color("234")).
		Width(w).Render(fmt.Sprintf("  %-22s  %s", "NAME", "MEMBERS"))

	total := len(m.clusters)
	viewStart, viewEnd := m.viewWindow(m.clCursor, total, listH)

	var listSb strings.Builder
	if total == 0 {
		listSb.WriteString(m.dimStyle.Render("  (no clusters defined)") + "\n")
	}
	for i := viewStart; i < viewEnd; i++ {
		cl := m.clusters[i]
		memberSummary := m.clusterMemberSummary(cl)
		line := fmt.Sprintf("%-22s  %s", cl.Name, memberSummary)
		if i == m.clCursor {
			listSb.WriteString(m.selectedStyle.Width(w).Render("> "+line) + "\n")
		} else {
			listSb.WriteString(m.normalStyle.Render("  "+line) + "\n")
		}
	}
	if total > listH {
		listSb.WriteString(m.dimStyle.Render(fmt.Sprintf("  %d/%d", m.clCursor+1, total)) + "\n")
	}

	body := "\n" + colHeader + "\n" + listSb.String()
	return m.renderScreen("/cluster", l1, body, "↑↓ navigate  Enter connect  a add  e edit  d delete  Esc back")
}

func (m Model) clusterMemberSummary(cl *model.Cluster) string {
	if len(cl.Members) == 0 {
		return "(no members)"
	}
	count := len(cl.Members)
	if count == 1 {
		mem := cl.Members[0]
		srv := serverByID(m.servers, mem.ServerID)
		if srv != nil {
			return srv.Host
		}
	}
	return fmt.Sprintf("%d members", count)
}

func (m Model) renderClusterForm() string {
	title := "Add Cluster"
	if m.clFormMode == fmEdit && m.clFormTarget != nil {
		title = "Edit  " + m.clFormTarget.Name
	}

	var body strings.Builder
	body.WriteString("\n")
	body.WriteString(m.formRow("Name", m.clFormName.View(), m.clFormName.Focused()))
	body.WriteString("\n")
	body.WriteString(m.dimStyle.Render("  Members (in order):") + "\n")

	if len(m.clFormMembers) == 0 {
		body.WriteString(m.dimStyle.Render("    (none — press 'a' to add)") + "\n")
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
			body.WriteString(m.selectedStyle.Render("> "+strings.TrimSpace(line)) + "\n")
		} else {
			body.WriteString(m.normalStyle.Render("  "+strings.TrimSpace(line)) + "\n")
		}
	}

	if m.clFormUserEditOpen {
		body.WriteString("\n")
		body.WriteString(m.dimStyle.Render("  User override:") + "\n")
		body.WriteString(fmt.Sprintf("  %s\n", m.clFormUserInput.View()))
	} else if m.clFormPickerOpen {
		body.WriteString(m.renderServerPicker(m.clFormPickerCursor))
	}

	hint := "↑↓ navigate  a add  x remove  u move-up  m move-down  r set-user  Tab name  Ctrl+S save  Esc cancel"
	if m.clFormName.Focused() {
		hint = "Tab member list  Ctrl+S save  Esc cancel"
	} else if m.clFormUserEditOpen {
		hint = "Enter confirm  Esc cancel"
	} else if m.clFormPickerOpen {
		hint = "↑↓ select  Enter add  Esc cancel"
	}
	bodyStr := body.String()
	if m.formEscConfirm {
		bodyStr = m.renderEscConfirmOverlay(bodyStr)
		hint = "Tab toggle  Enter confirm  Esc back to form"
	}
	return m.renderScreen(title, "cluster", bodyStr, hint)
}

// ── local hosts list/form ─────────────────────────────────────────────────────

func (m Model) renderHostList() string {
	l1, _ := pageDesc("hosts")
	listH := m.bodyHeight() - 3
	if listH < 1 {
		listH = 1
	}
	w := m.w()

	colHeader := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).Background(lipgloss.Color("234")).
		Width(w).Render(fmt.Sprintf("  %-28s  %-18s  %s", "HOSTNAME", "IP", "DESCRIPTION"))

	total := len(m.localHosts)
	viewStart, viewEnd := m.viewWindow(m.hostCursor, total, listH)

	var listSb strings.Builder
	if total == 0 {
		listSb.WriteString(m.dimStyle.Render("  (no local host mappings defined)") + "\n")
	}
	for i := viewStart; i < viewEnd; i++ {
		h := m.localHosts[i]
		desc := h.Description
		if desc == "" {
			desc = "—"
		}
		line := fmt.Sprintf("%-28s  %-18s  %s", h.Hostname, h.IP, desc)
		if i == m.hostCursor {
			listSb.WriteString(m.selectedStyle.Width(w).Render("> "+line) + "\n")
		} else {
			listSb.WriteString(m.normalStyle.Render("  "+line) + "\n")
		}
	}
	if total > listH {
		listSb.WriteString(m.dimStyle.Render(fmt.Sprintf("  %d/%d", m.hostCursor+1, total)) + "\n")
	}

	body := "\n" + colHeader + "\n" + listSb.String()
	return m.renderScreen("/hosts", l1, body, "↑↓ navigate  a add  e edit  d delete  Esc back")
}

func (m Model) renderHostForm() string {
	title := "Add Local Host"
	if m.hostFormMode == fmEdit && m.hostFormTarget != nil {
		title = "Edit  " + m.hostFormTarget.Hostname
	}

	var body strings.Builder
	body.WriteString("\n")
	labels := []string{"Hostname", "IP", "Description"}
	for i, field := range m.hostFormFields {
		lbl := labels[i]
		if m.hostFormMode == fmEdit && i == 0 {
			lbl += " (locked)"
		}
		body.WriteString(m.formRow(lbl, field.View(), m.hostFormFocus == i))
	}
	bodyStr := body.String()
	hint := "Tab next  Shift+Tab prev  Ctrl+S save  Esc cancel"
	if m.formEscConfirm {
		bodyStr = m.renderEscConfirmOverlay(bodyStr)
		hint = "Tab toggle  Enter confirm  Esc back to form"
	}
	return m.renderScreen(title, "local hosts", bodyStr, hint)
}

// ── tunnel list/form ──────────────────────────────────────────────────────────

func (m Model) renderTunnelList() string {
	l1, _ := pageDesc("tunnel")
	listH := m.bodyHeight() - 3
	if listH < 1 {
		listH = 1
	}
	w := m.w()

	colHeader := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).Background(lipgloss.Color("234")).
		Width(w).Render(fmt.Sprintf("  %-16s  %-14s  %-2s  %s",
			"NAME", "SERVER", "DIR", "FORWARD"))

	total := len(m.tunnels)
	viewStart, viewEnd := m.viewWindow(m.tunnelCursor, total, listH)

	var listSb strings.Builder
	if total == 0 {
		listSb.WriteString(m.dimStyle.Render("  (no tunnels configured)") + "\n")
	}
	for i := viewStart; i < viewEnd; i++ {
		t := m.tunnels[i]
		srv := serverByID(m.servers, t.ServerID)
		srvLabel := fmt.Sprintf("id=%d", t.ServerID)
		if srv != nil {
			srvLabel = srv.Host
		}
		running := m.tnStatuses[t.ID]
		statusTag := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render("●")
		if running {
			statusTag = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("●")
		}
		fwd := fmt.Sprintf("%s:%d → %s:%d", t.LocalHost, t.LocalPort, t.RemoteHost, t.RemotePort)
		line := fmt.Sprintf("%-16s  %-14s  %-2s  %s",
			t.Name, srvLabel, string(t.Direction), fwd)
		if i == m.tunnelCursor {
			listSb.WriteString(m.selectedStyle.Render("> "+line) + "  " + statusTag + "\n")
		} else {
			listSb.WriteString(m.normalStyle.Render("  "+line) + "  " + statusTag + "\n")
		}
	}
	if total > listH {
		listSb.WriteString(m.dimStyle.Render(fmt.Sprintf("  %d/%d", m.tunnelCursor+1, total)) + "\n")
	}

	body := "\n" + colHeader + "\n" + listSb.String()
	return m.renderScreen("/tunnel", l1, body, "↑↓ navigate  s start  x stop  a add  e edit  d delete  Esc back")
}

func (m Model) renderTunnelForm() string {
	title := "Add Tunnel"
	if m.tnFormMode == fmEdit && m.tnFormTarget != nil {
		title = "Edit  " + m.tnFormTarget.Name
	}

	var body strings.Builder
	body.WriteString("\n")
	body.WriteString(m.formRow("Name", m.tnFormFields[0].View(), m.tnFormFocus == 0))

	// Server picker row (tab stop 1)
	srvLabel := "(none — press Enter to pick)"
	if m.tnFormServerID != 0 {
		if srv := serverByID(m.servers, m.tnFormServerID); srv != nil {
			srvLabel = fmt.Sprintf("%s@%s", srv.User, srv.Host)
		} else {
			srvLabel = fmt.Sprintf("id=%d", m.tnFormServerID)
		}
	}
	body.WriteString(m.formRow("Server", srvLabel, m.tnFormFocus == tnFormIdxServer))
	if m.tnFormPickerOpen {
		body.WriteString(m.renderServerPicker(m.tnFormPickerCursor))
	}

	labels := []string{"Direction (L/R)", "Local Host", "Local Port", "Remote Host", "Remote Port"}
	for i, field := range m.tnFormFields[1:] {
		body.WriteString(m.formRow(labels[i], field.View(), m.tnFormFocus == i+2))
	}

	hint := "Tab next  Shift+Tab prev  Ctrl+S save  Esc cancel"
	if m.tnFormPickerOpen {
		hint = "↑↓ select server  Enter confirm  Esc cancel"
	}
	bodyStr := body.String()
	if m.formEscConfirm {
		bodyStr = m.renderEscConfirmOverlay(bodyStr)
		hint = "Tab toggle  Enter confirm  Esc back to form"
	}
	return m.renderScreen(title, "tunnel", bodyStr, hint)
}

// ── app-server list / form ────────────────────────────────────────────────────

func (m Model) renderAppServerList() string {
	listH := m.bodyHeight() - 3
	if listH < 1 {
		listH = 1
	}
	w := m.w()

	colHeader := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).Background(lipgloss.Color("234")).
		Width(w).Render(fmt.Sprintf("  %-20s  %-18s  %-14s  %s", "NAME", "SERVER", "PLUGIN", "DESCRIPTION"))

	total := len(m.appServers)
	viewStart, viewEnd := m.viewWindow(m.appServerCursor, total, listH)

	var listSb strings.Builder
	if total == 0 {
		listSb.WriteString(m.dimStyle.Render("  (no app-server bindings configured)") + "\n")
	}
	for i := viewStart; i < viewEnd; i++ {
		as := m.appServers[i]
		srvLabel := fmt.Sprintf("id=%d", as.ServerID)
		if srv := serverByID(m.servers, as.ServerID); srv != nil {
			srvLabel = srv.Host
		}
		line := fmt.Sprintf("%-20s  %-18s  %-14s  %s",
			as.Name, srvLabel, as.PluginName, as.Description)
		if i == m.appServerCursor {
			listSb.WriteString(m.selectedStyle.Width(w).Render("> "+line) + "\n")
		} else {
			listSb.WriteString(m.normalStyle.Render("  "+line) + "\n")
		}
	}
	if total > listH {
		listSb.WriteString(m.dimStyle.Render(fmt.Sprintf("  %d/%d", m.appServerCursor+1, total)) + "\n")
	}

	body := "\n" + colHeader + "\n" + listSb.String()
	return m.renderScreen("/app-server", "server+plugin bindings", body, "↑↓ navigate  Enter connect  a add  e edit  d delete  Esc back")
}

func (m Model) renderAppServerForm() string {
	title := "Add App-Server"
	if m.asFormMode == fmEdit && m.asFormTarget != nil {
		title = "Edit  " + m.asFormTarget.Name
	}

	var body strings.Builder
	body.WriteString("\n")
	labels := []string{"Name", "Plugin", "Description"}
	for i, field := range m.asFormFields {
		body.WriteString(m.formRow(labels[i], field.View(), m.asFormFocus == i))
	}

	// Server picker row
	srvLabel := "(none — press Enter to pick)"
	if m.asFormServerID != 0 {
		if srv := serverByID(m.servers, m.asFormServerID); srv != nil {
			srvLabel = fmt.Sprintf("%s@%s", srv.User, srv.Host)
		} else {
			srvLabel = fmt.Sprintf("id=%d", m.asFormServerID)
		}
	}
	body.WriteString(m.formRow("Server", srvLabel, m.asFormFocus == asFormIdxServer))
	if m.asFormPickerOpen {
		body.WriteString(m.renderServerPicker(m.asFormPickerCursor))
	}

	hint := "Tab next  Shift+Tab prev  Ctrl+S save  Esc cancel"
	if m.asFormPickerOpen {
		hint = "↑↓ select server  Enter confirm  Esc cancel"
	}
	bodyStr := body.String()
	if m.formEscConfirm {
		bodyStr = m.renderEscConfirmOverlay(bodyStr)
		hint = "Tab toggle  Enter confirm  Esc back to form"
	}
	return m.renderScreen(title, "app-server", bodyStr, hint)
}

// ── profile list / form ───────────────────────────────────────────────────────

func (m Model) renderProfileList() string {
	listH := m.bodyHeight() - 3
	if listH < 1 {
		listH = 1
	}
	w := m.w()

	colHeader := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).Background(lipgloss.Color("234")).
		Width(w).Render(fmt.Sprintf("  %-2s  %-20s  %-20s  %s", "✓", "NAME", "GATEWAY", "DESCRIPTION"))

	total := len(m.profiles)
	viewStart, viewEnd := m.viewWindow(m.profileCursor, total, listH)

	var listSb strings.Builder
	if total == 0 {
		listSb.WriteString(m.dimStyle.Render("  (no profiles configured)") + "\n")
	}
	for i := viewStart; i < viewEnd; i++ {
		p := m.profiles[i]
		activeMarker := "  "
		if p.IsActive {
			activeMarker = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("✓ ")
		}
		gwLabel := "—"
		if p.GatewayRouteID != nil {
			gwLabel = fmt.Sprintf("gw#%d", *p.GatewayRouteID)
			for _, gw := range m.gateways {
				if gw.ID == *p.GatewayRouteID {
					gwLabel = gw.Name
					break
				}
			}
		}
		line := fmt.Sprintf("%-20s  %-20s  %s", p.Name, gwLabel, p.Description)
		if i == m.profileCursor {
			listSb.WriteString(m.selectedStyle.Render("> "+activeMarker+line) + "\n")
		} else {
			listSb.WriteString(m.normalStyle.Render("  "+activeMarker+line) + "\n")
		}
	}
	if total > listH {
		listSb.WriteString(m.dimStyle.Render(fmt.Sprintf("  %d/%d", m.profileCursor+1, total)) + "\n")
	}

	body := "\n" + colHeader + "\n" + listSb.String()
	return m.renderScreen("/profile", "network profiles", body,
		"↑↓ navigate  Enter/u toggle-active  n deactivate  a add  e edit  d delete  Esc back")
}

func (m Model) renderProfileForm() string {
	title := "Add Profile"
	if m.pfFormMode == fmEdit && m.pfFormTarget != nil {
		title = "Edit  " + m.pfFormTarget.Name
	}

	var body strings.Builder
	body.WriteString("\n")
	labels := []string{"Name", "Description"}
	for i, field := range m.pfFormFields {
		body.WriteString(m.formRow(labels[i], field.View(), m.pfFormFocus == i))
	}

	// Gateway picker row
	gwLabel := "(none — press Enter to pick)"
	if m.pfFormGatewayID != nil {
		gwLabel = fmt.Sprintf("gw#%d", *m.pfFormGatewayID)
		for _, gw := range m.gateways {
			if gw.ID == *m.pfFormGatewayID {
				gwLabel = "[gw] " + gw.Name
				break
			}
		}
	}
	body.WriteString(m.formRow("Gateway", gwLabel, m.pfFormFocus == pfFormIdxGateway))

	if m.pfFormPickerOpen {
		entries := m.gwPickerEntries()
		body.WriteString("\n")
		body.WriteString(fmt.Sprintf("  Search: %s\n", m.srvFormGwSearch.View()))
		body.WriteString("\n")
		viewport := 8
		viewStart, viewEnd := m.viewWindow(m.pfFormPickerCursor, len(entries), viewport)
		for i := viewStart; i < viewEnd; i++ {
			e := entries[i]
			if i == m.pfFormPickerCursor {
				body.WriteString(m.selectedStyle.Render("  > " + e.label))
			} else {
				body.WriteString(m.normalStyle.Render("    " + e.label))
			}
			body.WriteString("\n")
		}
	}

	hint := "Tab next  Shift+Tab prev  Ctrl+S save  Esc cancel"
	if m.pfFormPickerOpen {
		hint = "↑↓ navigate  type to search  Enter select  Esc close picker"
	}
	bodyStr := body.String()
	if m.formEscConfirm {
		bodyStr = m.renderEscConfirmOverlay(bodyStr)
		hint = "Tab toggle  Enter confirm  Esc back to form"
	}
	return m.renderScreen(title, "profile", bodyStr, hint)
}

// ── plugin picker ─────────────────────────────────────────────────────────────

func (m Model) renderPluginPicker() string {
	var body strings.Builder
	body.WriteString("\n")

	if len(m.plugins) == 0 {
		body.WriteString(m.dimStyle.Render("  No plugins found in: "+m.pluginDir) + "\n")
		body.WriteString(m.dimStyle.Render("  Add YAML files to enable app plugins.") + "\n")
	} else {
		for i, name := range m.plugins {
			if i == m.pluginCursor {
				body.WriteString(m.selectedStyle.Render("> "+name) + "\n")
			} else {
				body.WriteString(m.normalStyle.Render("  "+name) + "\n")
			}
		}
	}
	return m.renderScreen("Select Plugin", "app plugins", body.String(), "↑↓ navigate  Enter select  Esc cancel")
}

// ── plugin list ───────────────────────────────────────────────────────────────

func (m Model) renderPluginList() string {
	listH := m.bodyHeight() - 3
	if listH < 1 {
		listH = 1
	}
	w := m.w()

	colHeader := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).Background(lipgloss.Color("234")).
		Width(w).Render(fmt.Sprintf("  %-20s  %-8s  %-12s  %s", "NAME", "VERSION", "AUTH", "RUNTIME"))

	var listSb strings.Builder
	if len(m.pluginList) == 0 {
		listSb.WriteString(m.dimStyle.Render("  No plugins found.") + "\n")
		listSb.WriteString(m.dimStyle.Render("  Add YAML files to: "+m.pluginDir) + "\n")
	} else {
		total := len(m.pluginList)
		viewStart, viewEnd := m.viewWindow(m.pluginListCursor, total, listH)
		for i := viewStart; i < viewEnd; i++ {
			p := m.pluginList[i]
			strategies := strings.Join(p.Runtime.Strategies, ",")
			line := fmt.Sprintf("%-20s  %-8s  %-12s  %s",
				p.Name, "v"+p.Version, string(p.Auth.Provider), strategies)
			if i == m.pluginListCursor {
				listSb.WriteString(m.selectedStyle.Width(w).Render("> "+line) + "\n")
			} else {
				listSb.WriteString(m.normalStyle.Render("  "+line) + "\n")
			}
		}
		if total > listH {
			listSb.WriteString(m.dimStyle.Render(fmt.Sprintf("  %d/%d", m.pluginListCursor+1, total)) + "\n")
		}
	}

	body := "\n" + colHeader + "\n" + listSb.String()
	return m.renderScreen("/plugin", "application plugins", body, "↑↓ navigate  Enter detail  Esc back")
}

// ── plugin detail ─────────────────────────────────────────────────────────────

func (m Model) renderPluginDetail() string {
	p := m.pluginDetail
	if p == nil {
		return m.renderScreen("Plugin Detail", "", m.dimStyle.Render("  No plugin selected."), "Esc back")
	}

	accent := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
	dim := m.dimStyle

	var body strings.Builder
	body.WriteString("\n")
	body.WriteString(accent.Render("  "+p.Name) +
		dim.Render(fmt.Sprintf("  v%s", p.Version)) + "\n\n")

	body.WriteString(label.Render("  Auth") +
		dim.Render(fmt.Sprintf("  provider: %s", p.Auth.Provider)) + "\n")
	for _, mv := range p.Auth.Mapping {
		body.WriteString(fmt.Sprintf("    %s  expose:%-8s  path: %s\n",
			accent.Render(mv.Var), string(mv.Expose), mv.Path))
		if mv.Automation != nil {
			body.WriteString(dim.Render(fmt.Sprintf("      expect: %q  send_newline: %v\n",
				mv.Automation.Expect, mv.Automation.SendNewline)))
		}
	}
	body.WriteString("\n")

	body.WriteString(label.Render("  Runtime") +
		dim.Render(fmt.Sprintf("  strategies: %s", strings.Join(p.Runtime.Strategies, " → "))) + "\n")
	if n := p.Runtime.Environments.Native; n != nil {
		body.WriteString(fmt.Sprintf("    native   %s %s\n",
			accent.Render(n.Command), dim.Render(strings.Join(n.Args, " "))))
	}
	if d := p.Runtime.Environments.Docker; d != nil {
		body.WriteString(fmt.Sprintf("    docker   match:%s  %s %s\n",
			d.ContainerMatch, accent.Render(d.Command), dim.Render(strings.Join(d.Args, " "))))
	}

	return m.renderScreen(p.Name, "plugin detail", body.String(), "e edit in $EDITOR  Esc back")
}

// ── shared helpers ────────────────────────────────────────────────────────────

// renderEscConfirmOverlay overlays the "Save before leaving?" dialog on top of body.
// It appends the dialog lines after the body content.
func (m Model) renderEscConfirmOverlay(body string) string {
	cancelStyle := lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("255"))
	saveStyle := lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("255"))
	cancelSel := lipgloss.NewStyle().Padding(0, 2).
		Bold(true).Foreground(lipgloss.Color("196")).Background(lipgloss.Color("236"))
	saveSel := lipgloss.NewStyle().Padding(0, 2).
		Bold(true).Foreground(lipgloss.Color("82")).Background(lipgloss.Color("236"))

	cancelBtn := cancelStyle.Render("[ Cancel ]")
	saveBtn := saveStyle.Render("[ Save ]")
	if !m.formEscConfirmSave {
		cancelBtn = cancelSel.Render("[ Cancel ]")
	} else {
		saveBtn = saveSel.Render("[ Save ]")
	}

	w := m.w()
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")).
		Padding(1, 3).
		Width(min(w-8, 44))

	question := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).
		Render("Save changes before leaving?")
	buttons := "  " + cancelBtn + "    " + saveBtn
	box := boxStyle.Render(question + "\n\n" + buttons)

	padLeft := (w - lipgloss.Width(box)) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	pad := strings.Repeat(" ", padLeft)
	var overlayLines []string
	for _, line := range strings.Split(box, "\n") {
		overlayLines = append(overlayLines, pad+line)
	}
	return body + "\n" + strings.Join(overlayLines, "\n")
}

// formRow renders one labeled form field.
func (m Model) formRow(label, value string, focused bool) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Width(16)
	if focused {
		labelStyle = labelStyle.Foreground(lipgloss.Color("212")).Bold(true)
	}
	cursor := "  "
	if focused {
		cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true).Render("> ")
	}
	return cursor + labelStyle.Render(label) + "  " + value + "\n"
}

func (m Model) renderServerPicker(cursor int) string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(m.dimStyle.Render("  Pick server:") + "\n")
	viewport := 8
	total := len(m.servers)
	viewStart, viewEnd := m.viewWindow(cursor, total, viewport)
	for i := viewStart; i < viewEnd; i++ {
		s := m.servers[i]
		line := fmt.Sprintf("%s@%s", s.User, s.Host)
		if i == cursor {
			sb.WriteString(m.selectedStyle.Render("  > " + line))
		} else {
			sb.WriteString(m.normalStyle.Render("    " + line))
		}
		sb.WriteString("\n")
	}
	if total > viewport {
		sb.WriteString(m.dimStyle.Render(fmt.Sprintf("    %d/%d", cursor+1, total)) + "\n")
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

// visibleRows returns how many list rows fit in the terminal.
// Kept for backward compat — renderScreen-based views use bodyHeight() directly.
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

// viewWindow returns the [start, end) slice indices that keep cursor visible.
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

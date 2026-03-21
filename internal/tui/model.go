// Package tui provides the interactive host selector built with Bubbletea.
package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/emusal/alogin2/internal/db"
	"github.com/emusal/alogin2/internal/model"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SelectedServer is the result returned when the user picks a server.
type SelectedServer struct {
	Server *model.Server
	User   string
}

// state tracks what the TUI is currently doing.
type state int

const (
	stateWelcome       state = iota // landing/welcome screen
	stateList                       // server list + fuzzy search
	stateDetail                     // server detail panel
	stateServerForm                 // add/edit server form
	stateConfirmDelete              // delete confirmation
	stateGatewayList                // gateway management list
	stateGatewayForm                // add/edit gateway form
	stateClusterList                // cluster management list
	stateClusterForm                // add/edit cluster form
	stateHostList                   // local hosts management list
	stateHostForm                   // add/edit local host form
)

// StartAt specifies which section to open when launching the TUI.
type StartAt int

const (
	StartAtWelcome StartAt = iota // show welcome/home screen (default)
	StartAtList                   // jump directly to server list
	StartAtGateway                // jump to gateway management
	StartAtCluster                // jump to cluster management
	StartAtHosts                  // jump to local hosts management
)

type formMode int

const (
	fmAdd  formMode = iota
	fmEdit
)

// tuiCommand is a slash-command shown in the command palette.
type tuiCommand struct {
	trigger string // e.g. "/gateway"
	desc    string
}

var globalCommands = []tuiCommand{
	{"/server",  "Manage servers"},
	{"/gateway", "Manage gateways"},
	{"/cluster", "Manage clusters"},
	{"/hosts",   "Manage local hostname mappings"},
}

// memberEntry tracks one cluster member in the form.
type memberEntry struct {
	serverID int64
	user     string // empty = use server default
}

// Model is the Bubbletea model for the host selector.
type Model struct {
	// Data
	servers  []*model.Server
	filtered []*model.Server
	gateways []*model.GatewayRoute
	clusters []*model.Cluster
	db       *db.DB

	// Startup configuration
	startAt StartAt

	// Terminal dimensions
	termWidth  int
	termHeight int

	// Welcome screen stats (loaded async)
	gwCount int
	clCount int

	// List state
	cursor   int
	query    string
	state    state
	choice   *SelectedServer
	quitting bool

	// Slash-command palette (active when query starts with "/")
	cmdCursor int

	// Server form
	formMode     formMode
	formFields   []textinput.Model
	formFocusIdx int
	formTarget   *model.Server

	// Delete confirm
	deleteTarget *model.Server

	// Gateway list
	gatewayCursor int

	// Gateway form
	gwFormMode         formMode
	gwFormName         textinput.Model
	gwFormHops         []int64
	gwFormHopCursor    int
	gwFormPickerOpen   bool
	gwFormPickerCursor int
	gwFormTarget       *model.GatewayRoute

	// Cluster list
	clCursor int

	// Cluster form
	clFormMode         formMode
	clFormName         textinput.Model
	clFormMembers      []memberEntry
	clFormMemberCursor int
	clFormPickerOpen   bool
	clFormPickerCursor int
	clFormUserEditOpen bool
	clFormUserInput    textinput.Model
	clFormTarget       *model.Cluster

	// Local hosts list
	localHosts       []*model.LocalHost
	hostCursor       int

	// Local host form
	hostFormMode   formMode
	hostFormFields []textinput.Model // [0]=hostname [1]=ip [2]=description
	hostFormFocus  int
	hostFormTarget *model.LocalHost

	// Status/error message
	statusMsg string

	// Styles
	titleStyle    lipgloss.Style
	selectedStyle lipgloss.Style
	normalStyle   lipgloss.Style
	dimStyle      lipgloss.Style
	inputStyle    lipgloss.Style
	detailStyle   lipgloss.Style
	formStyle     lipgloss.Style
	confirmStyle  lipgloss.Style
	cmdStyle      lipgloss.Style
}

// NewModel creates a TUI model starting at the welcome screen.
func NewModel(servers []*model.Server, database *db.DB) Model {
	return NewModelAt(servers, database, StartAtWelcome)
}

// NewModelAt creates a TUI model starting at the given section.
func NewModelAt(servers []*model.Server, database *db.DB, start StartAt) Model {
	initialState := stateWelcome
	switch start {
	case StartAtList:
		initialState = stateList
	case StartAtGateway:
		initialState = stateGatewayList
	case StartAtCluster:
		initialState = stateClusterList
	case StartAtHosts:
		initialState = stateHostList
	}

	m := Model{
		servers:  servers,
		filtered: servers,
		db:       database,
		startAt:  start,
		state:    initialState,
	}

	m.titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		Padding(0, 1)

	m.selectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("212")).
		Background(lipgloss.Color("236")).
		Bold(true).
		Padding(0, 1)

	m.normalStyle = lipgloss.NewStyle().
		Padding(0, 1)

	m.dimStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Padding(0, 1)

	m.inputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("212")).
		Padding(0, 1)

	m.detailStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		Width(50)

	m.formStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("212")).
		Padding(1, 2).
		Width(60)

	m.confirmStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Padding(1, 2).
		Width(50)

	m.cmdStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("226")).
		Padding(0, 1).
		Width(36)

	return m
}

// Choice returns the selected server, or nil if none was chosen.
func (m Model) Choice() *SelectedServer { return m.choice }

// Init implements tea.Model — triggers async data loading based on start section.
func (m Model) Init() tea.Cmd {
	switch m.startAt {
	case StartAtGateway:
		return m.loadGatewaysCmd()
	case StartAtCluster:
		return m.loadClustersCmd()
	case StartAtHosts:
		return m.loadHostsCmd()
	default:
		return m.loadStatsCmd()
	}
}

// loadStatsCmd loads gateway and cluster counts for the welcome screen.
func (m Model) loadStatsCmd() tea.Cmd {
	return func() tea.Msg {
		gws, _ := m.db.Gateways.ListAll(context.Background())
		cls, _ := m.db.Clusters.ListAll(context.Background())
		return statsMsg{gateways: gws, clusters: cls}
	}
}

// loadHostsCmd loads the full local hosts list.
func (m Model) loadHostsCmd() tea.Cmd {
	return func() tea.Msg {
		hosts, err := m.db.Hosts.ListAll(context.Background())
		if err != nil {
			return hostErrMsg{err: err}
		}
		return hostDoneMsg{hosts: hosts}
	}
}

// filteredCommands returns commands matching the current slash query.
func (m Model) filteredCommands() []tuiCommand {
	q := strings.TrimPrefix(m.query, "/")
	if q == "" {
		return globalCommands
	}
	var out []tuiCommand
	for _, c := range globalCommands {
		if strings.HasPrefix(strings.TrimPrefix(c.trigger, "/"), q) {
			out = append(out, c)
		}
	}
	return out
}

// ── server form ──────────────────────────────────────────────────────────────

func (m *Model) initServerForm(srv *model.Server) {
	fields := make([]textinput.Model, 7)
	for i := range fields {
		fields[i] = textinput.New()
		fields[i].CharLimit = 256
	}
	fields[0].Placeholder = "ssh"
	fields[1].Placeholder = "hostname or IP"
	fields[2].Placeholder = "username"
	fields[3].EchoMode = textinput.EchoPassword
	fields[3].Placeholder = "(leave empty = keep current)"
	fields[4].Placeholder = "0"
	fields[4].CharLimit = 5
	fields[5].Placeholder = "gateway name (optional)"
	fields[6].Placeholder = "e.g. ko_KR.eucKR"

	if srv != nil {
		fields[0].SetValue(string(srv.Protocol))
		fields[1].SetValue(srv.Host)
		fields[2].SetValue(srv.User)
		if srv.Port > 0 {
			fields[4].SetValue(strconv.Itoa(srv.Port))
		}
		if srv.GatewayID != nil {
			for _, gw := range m.gateways {
				if gw.ID == *srv.GatewayID {
					fields[5].SetValue(gw.Name)
					break
				}
			}
		}
		fields[6].SetValue(srv.Locale)
		m.formTarget = srv
		m.formMode = fmEdit
	} else {
		m.formMode = fmAdd
		m.formTarget = nil
		fields[0].SetValue("ssh")
	}

	fields[0].Focus()
	m.formFields = fields
	m.formFocusIdx = 0
	m.state = stateServerForm
	m.statusMsg = ""
}

func (m Model) submitServerForm() tea.Cmd {
	return func() tea.Msg {
		proto := model.Protocol(m.formFields[0].Value())
		if proto == "" {
			proto = model.ProtoSSH
		}
		host := m.formFields[1].Value()
		user := m.formFields[2].Value()
		password := m.formFields[3].Value()
		portStr := m.formFields[4].Value()
		gwName := m.formFields[5].Value()
		locale := m.formFields[6].Value()

		port, _ := strconv.Atoi(portStr)

		srv := &model.Server{
			Protocol: proto,
			Host:     host,
			User:     user,
			Port:     port,
			Locale:   locale,
		}

		if gwName != "" {
			gw, err := m.db.Gateways.GetByName(context.Background(), gwName)
			if err == nil && gw != nil {
				srv.GatewayID = &gw.ID
			}
		}

		var opErr error
		if m.formMode == fmAdd {
			opErr = m.db.Servers.Create(context.Background(), srv, password)
		} else {
			srv.ID = m.formTarget.ID
			srv.Host = m.formTarget.Host
			opErr = m.db.Servers.Update(context.Background(), srv, password)
		}
		if opErr != nil {
			return formErrMsg{opErr}
		}

		servers, _ := m.db.Servers.ListAll(context.Background())
		return formDoneMsg{servers}
	}
}

func (m Model) deleteServerCmd() tea.Cmd {
	return func() tea.Msg {
		if err := m.db.Servers.Delete(context.Background(), m.deleteTarget.ID); err != nil {
			return formErrMsg{err}
		}
		servers, _ := m.db.Servers.ListAll(context.Background())
		return formDoneMsg{servers}
	}
}

// ── gateway form ─────────────────────────────────────────────────────────────

func (m *Model) initGatewayForm(gw *model.GatewayRoute) {
	inp := textinput.New()
	inp.CharLimit = 128
	inp.Placeholder = "gateway name"

	if gw != nil {
		inp.SetValue(gw.Name)
		hops := make([]int64, len(gw.Hops))
		for i, h := range gw.Hops {
			hops[i] = h.ServerID
		}
		m.gwFormHops = hops
		m.gwFormTarget = gw
		m.gwFormMode = fmEdit
	} else {
		m.gwFormHops = nil
		m.gwFormTarget = nil
		m.gwFormMode = fmAdd
	}

	inp.Focus()
	m.gwFormName = inp
	m.gwFormHopCursor = 0
	m.gwFormPickerOpen = false
	m.gwFormPickerCursor = 0
	m.state = stateGatewayForm
	m.statusMsg = ""
}

func (m Model) submitGatewayForm() tea.Cmd {
	return func() tea.Msg {
		name := m.gwFormName.Value()
		if name == "" {
			return gwErrMsg{fmt.Errorf("name is required")}
		}
		var opErr error
		if m.gwFormMode == fmAdd {
			_, opErr = m.db.Gateways.Create(context.Background(), name, m.gwFormHops)
		} else {
			_, opErr = m.db.Gateways.Update(context.Background(), m.gwFormTarget.ID, name, m.gwFormHops)
		}
		if opErr != nil {
			return gwErrMsg{opErr}
		}
		gateways, _ := m.db.Gateways.ListAll(context.Background())
		return gwDoneMsg{gateways, "Saved."}
	}
}

func (m Model) deleteGatewayCmd(id int64) tea.Cmd {
	return func() tea.Msg {
		if err := m.db.Gateways.Delete(context.Background(), id); err != nil {
			return gwErrMsg{err}
		}
		gateways, _ := m.db.Gateways.ListAll(context.Background())
		return gwDoneMsg{gateways, "Deleted."}
	}
}

func (m Model) loadGatewaysCmd() tea.Cmd {
	return func() tea.Msg {
		gateways, _ := m.db.Gateways.ListAll(context.Background())
		return gwDoneMsg{gateways, ""}
	}
}

// ── cluster form ─────────────────────────────────────────────────────────────

func (m *Model) initClusterForm(cl *model.Cluster) {
	inp := textinput.New()
	inp.CharLimit = 128
	inp.Placeholder = "cluster name"

	if cl != nil {
		inp.SetValue(cl.Name)
		members := make([]memberEntry, len(cl.Members))
		for i, mem := range cl.Members {
			members[i] = memberEntry{serverID: mem.ServerID, user: mem.User}
		}
		m.clFormMembers = members
		m.clFormTarget = cl
		m.clFormMode = fmEdit
	} else {
		m.clFormMembers = nil
		m.clFormTarget = nil
		m.clFormMode = fmAdd
	}

	inp.Focus()
	m.clFormName = inp
	m.clFormMemberCursor = 0
	m.clFormPickerOpen = false
	m.clFormPickerCursor = 0
	m.clFormUserEditOpen = false
	m.clFormUserInput = textinput.New()
	m.clFormUserInput.CharLimit = 64
	m.clFormUserInput.Placeholder = "user override (empty = default)"
	m.state = stateClusterForm
	m.statusMsg = ""
}

func (m Model) submitClusterForm() tea.Cmd {
	return func() tea.Msg {
		name := m.clFormName.Value()
		if name == "" {
			return clErrMsg{fmt.Errorf("name is required")}
		}
		members := make([]model.ClusterMember, len(m.clFormMembers))
		for i, mem := range m.clFormMembers {
			members[i] = model.ClusterMember{ServerID: mem.serverID, User: mem.user, MemberOrder: i}
		}
		var opErr error
		if m.clFormMode == fmAdd {
			_, opErr = m.db.Clusters.Create(context.Background(), name, members)
		} else {
			_, opErr = m.db.Clusters.Update(context.Background(), m.clFormTarget.ID, name, members)
		}
		if opErr != nil {
			return clErrMsg{opErr}
		}
		clusters, _ := m.db.Clusters.ListAll(context.Background())
		return clDoneMsg{clusters, "Saved."}
	}
}

func (m Model) deleteClusterCmd(id int64) tea.Cmd {
	return func() tea.Msg {
		if err := m.db.Clusters.Delete(context.Background(), id); err != nil {
			return clErrMsg{err}
		}
		clusters, _ := m.db.Clusters.ListAll(context.Background())
		return clDoneMsg{clusters, "Deleted."}
	}
}

func (m Model) loadClustersCmd() tea.Cmd {
	return func() tea.Msg {
		clusters, _ := m.db.Clusters.ListAll(context.Background())
		return clDoneMsg{clusters, ""}
	}
}

// ── local host form ───────────────────────────────────────────────────────────

func (m *Model) initHostForm(h *model.LocalHost) {
	fields := make([]textinput.Model, 3)
	for i := range fields {
		fields[i] = textinput.New()
		fields[i].CharLimit = 256
	}
	fields[0].Placeholder = "hostname  (e.g. myserver)"
	fields[1].Placeholder = "IP address  (e.g. 192.168.1.10)"
	fields[2].Placeholder = "description (optional)"

	if h != nil {
		fields[0].SetValue(h.Hostname)
		fields[1].SetValue(h.IP)
		fields[2].SetValue(h.Description)
		m.hostFormTarget = h
		m.hostFormMode = fmEdit
		fields[1].Focus() // hostname is immutable on edit
		m.hostFormFocus = 1
	} else {
		m.hostFormTarget = nil
		m.hostFormMode = fmAdd
		fields[0].Focus()
		m.hostFormFocus = 0
	}

	m.hostFormFields = fields
	m.state = stateHostForm
	m.statusMsg = ""
}

func (m Model) submitHostForm() tea.Cmd {
	return func() tea.Msg {
		hostname := m.hostFormFields[0].Value()
		ip := m.hostFormFields[1].Value()
		desc := m.hostFormFields[2].Value()
		if hostname == "" || ip == "" {
			return hostErrMsg{fmt.Errorf("hostname and IP are required")}
		}
		h := &model.LocalHost{Hostname: hostname, IP: ip, Description: desc}
		var opErr error
		if m.hostFormMode == fmAdd {
			opErr = m.db.Hosts.Create(context.Background(), h)
		} else {
			h.ID = m.hostFormTarget.ID
			h.Hostname = m.hostFormTarget.Hostname
			opErr = m.db.Hosts.Update(context.Background(), h)
		}
		if opErr != nil {
			return hostErrMsg{opErr}
		}
		hosts, _ := m.db.Hosts.ListAll(context.Background())
		return hostDoneMsg{hosts, "Saved."}
	}
}

func (m Model) deleteHostCmd(id int64) tea.Cmd {
	return func() tea.Msg {
		if err := m.db.Hosts.Delete(context.Background(), id); err != nil {
			return hostErrMsg{err}
		}
		hosts, _ := m.db.Hosts.ListAll(context.Background())
		return hostDoneMsg{hosts, "Deleted."}
	}
}

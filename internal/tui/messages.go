package tui

import (
	"github.com/emusal/alogin2/internal/model"
	pluginpkg "github.com/emusal/alogin2/internal/plugin"
)

type asDoneMsg struct {
	appServers []*model.AppServer
	msg        string
}
type asErrMsg struct{ err error }

// pluginLoadedMsg is sent when the plugin list has been loaded from disk.
type pluginLoadedMsg struct{ plugins []string }

// pluginListLoadedMsg carries full Plugin objects for the /plugin browser.
type pluginListLoadedMsg struct{ plugins []*pluginpkg.Plugin }

// pluginReloadedMsg is sent after $EDITOR closes — reloads both the list and the detail.
type pluginReloadedMsg struct {
	plugins []*pluginpkg.Plugin
	detail  *pluginpkg.Plugin // may be nil if file was deleted or parse failed
}

type formDoneMsg struct{ servers []*model.Server }
type formErrMsg struct{ err error }

type gwDoneMsg struct {
	gateways []*model.GatewayRoute
	msg      string
}
type gwErrMsg struct{ err error }

type clDoneMsg struct {
	clusters []*model.Cluster
	msg      string
}
type clErrMsg struct{ err error }

type hostDoneMsg struct {
	hosts []*model.LocalHost
	msg   string
}
type hostErrMsg struct{ err error }

// statsMsg carries the full gateway and cluster lists loaded at startup.
type statsMsg struct {
	gateways []*model.GatewayRoute
	clusters []*model.Cluster
}

// gwLoadedMsg silently refreshes m.gateways without changing TUI state.
// Used when opening the server form gateway picker.
type gwLoadedMsg struct{ gateways []*model.GatewayRoute }

type tnDoneMsg struct {
	tunnels []*model.Tunnel
	msg     string
}
type tnErrMsg struct{ err error }

type tnStatusMsg struct{ statuses map[int64]bool }

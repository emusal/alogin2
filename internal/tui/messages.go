package tui

import "github.com/emusal/alogin2/internal/model"

type formDoneMsg struct{ servers []*model.Server }
type formErrMsg  struct{ err error }

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

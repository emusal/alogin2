// Package terminal bridges a WebSocket connection to an SSH PTY session.
package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/emusal/alogin2/internal/db"
	"github.com/emusal/alogin2/internal/model"
	"github.com/emusal/alogin2/internal/plugin"
	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/emusal/alogin2/internal/vault"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	gossh "golang.org/x/crypto/ssh"
)

// Handler handles WebSocket terminal connections.
type Handler struct {
	db        *db.DB
	vlt       vault.Vault
	pluginDir string
}

// NewHandler creates a terminal WebSocket handler.
func NewHandler(database *db.DB, vlt vault.Vault) *Handler {
	return &Handler{db: database, vlt: vlt}
}

// WithPluginDir sets the plugin directory for app-aware terminal sessions.
func (h *Handler) WithPluginDir(dir string) *Handler {
	h.pluginDir = dir
	return h
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // restrict in production
}

// wsMessage is the JSON envelope sent over the WebSocket.
type wsMessage struct {
	Type string `json:"type"` // "data", "resize", "ping"
	Data string `json:"data"` // terminal data (UTF-8)
	Cols int    `json:"cols"` // for resize
	Rows int    `json:"rows"` // for resize
}

// ServeWS upgrades an HTTP request to a WebSocket and bridges it to an SSH PTY.
func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	serverIDStr := chi.URLParam(r, "serverID")
	if serverIDStr == "" {
		serverIDStr = r.URL.Query().Get("serverID")
	}
	serverID, err := strconv.ParseInt(serverIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid serverID", http.StatusBadRequest)
		return
	}

	profileOverride := r.URL.Query().Get("profile")
	appName := r.URL.Query().Get("app")

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	if err := h.runSession(r.Context(), ws, serverID, profileOverride, appName); err != nil {
		msg, _ := json.Marshal(wsMessage{Type: "data", Data: "\r\nError: " + err.Error() + "\r\n"})
		ws.WriteMessage(websocket.TextMessage, msg)
	}
}

func (h *Handler) runSession(ctx context.Context, ws *websocket.Conn, serverID int64, profileOverride string, appName string) error {
	srv, err := h.db.Servers.GetByID(ctx, serverID)
	if err != nil || srv == nil {
		return fmt.Errorf("server %d not found", serverID)
	}

	hops, err := h.buildHops(ctx, srv, profileOverride)
	if err != nil {
		return err
	}

	chain, err := internalssh.DialChain(hops)
	if err != nil {
		return err
	}
	defer chain.CloseAll()

	client := chain.Terminal()
	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	// Request PTY
	if err := sess.RequestPty("xterm-256color", 24, 80, gossh.TerminalModes{
		gossh.ECHO:          1,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		return fmt.Errorf("pty: %w", err)
	}

	stdinPipe, err := sess.StdinPipe()
	if err != nil {
		return err
	}
	stdoutPipe, err := sess.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := sess.StderrPipe()
	if err != nil {
		return err
	}

	var ptyRules []plugin.PTYRule
	if appName != "" && h.pluginDir != "" {
		p, loadErr := plugin.LoadFromFile(filepath.Join(h.pluginDir, appName+".yaml"))
		if loadErr == nil && p != nil {
			runner := &wsRunner{client: client}
			if pluginSess, prepErr := plugin.Prepare(ctx, p, h.vlt, runner); prepErr == nil {
				ptyRules = pluginSess.BuildPTYRules()
				cmd := pluginSess.Strategy.BuildCommand()
				if err := sess.Start(cmd); err != nil {
					return fmt.Errorf("start plugin %s: %w", appName, err)
				}
				goto sessionStarted
			}
		}
	}
	if err := sess.Shell(); err != nil {
		return err
	}
sessionStarted:

	var wg sync.WaitGroup

	// SSH stdout → WebSocket (+ PTY rule automation)
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		// pending holds unmatched output for expect scanning across reads.
		var pending bytes.Buffer
		for {
			n, readErr := stdoutPipe.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				msg, _ := json.Marshal(wsMessage{Type: "data", Data: string(chunk)})
				ws.WriteMessage(websocket.TextMessage, msg)

				// Expect-Send automation: check pending+chunk for each rule.
				if len(ptyRules) > 0 {
					pending.Write(chunk)
					combined := pending.String()
					for i := range ptyRules {
						r := &ptyRules[i]
						if r.Pattern != "" && strings.Contains(combined, r.Pattern) {
							send := r.Send
							if r.SendNewline {
								send += "\n"
							}
							stdinPipe.Write([]byte(send))
							r.Pattern = "" // fire once
							pending.Reset()
						}
					}
					// Keep only the last 512 bytes to bound memory.
					if pending.Len() > 512 {
						tail := pending.Bytes()
						pending.Reset()
						pending.Write(tail[len(tail)-512:])
					}
				}
			}
			if readErr != nil {
				return
			}
		}
	}()

	// SSH stderr → discard (merged via PTY)
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(io.Discard, stderrPipe)
	}()

	// WebSocket → SSH stdin
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stdinPipe.Close()
		for {
			_, rawMsg, err := ws.ReadMessage()
			if err != nil {
				return
			}
			var msg wsMessage
			if err := json.Unmarshal(rawMsg, &msg); err != nil {
				stdinPipe.Write(rawMsg)
				continue
			}
			switch msg.Type {
			case "data":
				stdinPipe.Write([]byte(msg.Data))
			case "resize":
				sess.WindowChange(msg.Rows, msg.Cols)
			}
		}
	}()

	sess.Wait()
	wg.Wait()
	return nil
}

// resolveHost looks up the hostname in the local_hosts table; returns as-is if not found.
func (h *Handler) resolveHost(ctx context.Context, hostname string) string {
	return h.db.Hosts.Resolve(ctx, hostname)
}

// buildHops constructs the SSH hop list using the active network profile.
// profileOverride: "" = active profile, "none" = direct, "<name>" = named profile.
// Loop prevention: if the destination is one of the gateway hops, connect directly.
func (h *Handler) buildHops(ctx context.Context, srv *model.Server, profileOverride string) ([]internalssh.HopConfig, error) {
	var hops []internalssh.HopConfig
	var activeProfile *model.Profile

	if profileOverride != "none" {
		var err error

		if profileOverride != "" {
			activeProfile, err = h.db.Profiles.GetByName(ctx, profileOverride)
			if err != nil {
				return nil, fmt.Errorf("profile %q: %w", profileOverride, err)
			}
			if activeProfile == nil {
				return nil, fmt.Errorf("profile %q not found", profileOverride)
			}
		} else {
			activeProfile, err = h.db.Profiles.GetActive(ctx)
			if err != nil {
				return nil, fmt.Errorf("get active profile: %w", err)
			}
		}

		if activeProfile != nil && activeProfile.GatewayRouteID != nil {
			gw, err := h.db.Gateways.GetByID(ctx, *activeProfile.GatewayRouteID)
			if err != nil || gw == nil {
				return nil, fmt.Errorf("profile gateway route not found: %w", err)
			}

			// Loop prevention: if destination is one of the gateway hops, connect directly.
			isGWHop := false
			for _, gh := range gw.Hops {
				if gh.ServerID == srv.ID {
					isGWHop = true
					break
				}
			}

			if !isGWHop {
				for _, gh := range gw.Hops {
					hopSrv, err := h.db.Servers.GetByID(ctx, gh.ServerID)
					if err != nil || hopSrv == nil {
						return nil, fmt.Errorf("gateway hop server %d not found", gh.ServerID)
					}
					pwd, _ := h.vlt.Get(hopSrv.User + "@" + hopSrv.Host)
					hops = append(hops, internalssh.HopConfig{
						Host:       h.resolveHost(ctx, hopSrv.Host),
						Port:       hopSrv.EffectivePort(),
						User:       hopSrv.User,
						Password:   pwd,
						AuthMethod: hopSrv.AuthMethod,
						KeyPath:    hopSrv.IdentityFile,
					})
				}
			}
		}
	}

	// Server-level internal gateway hops (applied after profile gateway, before destination).
	// Dedup: if server gateway is the same route as the profile gateway, skip it.
	if srv.GatewayRouteID != nil && (activeProfile == nil ||
		activeProfile.GatewayRouteID == nil ||
		*srv.GatewayRouteID != *activeProfile.GatewayRouteID) {
		srvGW, err := h.db.Gateways.GetByID(ctx, *srv.GatewayRouteID)
		if err != nil || srvGW == nil {
			return nil, fmt.Errorf("server gateway route %d not found", *srv.GatewayRouteID)
		}
		for _, gh := range srvGW.Hops {
			if gh.ServerID == srv.ID {
				continue
			}
			hopSrv, err := h.db.Servers.GetByID(ctx, gh.ServerID)
			if err != nil || hopSrv == nil {
				return nil, fmt.Errorf("server gateway hop %d not found", gh.ServerID)
			}
			pwd, _ := h.vlt.Get(hopSrv.User + "@" + hopSrv.Host)
			hops = append(hops, internalssh.HopConfig{
				Host:       h.resolveHost(ctx, hopSrv.Host),
				Port:       hopSrv.EffectivePort(),
				User:       hopSrv.User,
				Password:   pwd,
				AuthMethod: hopSrv.AuthMethod,
				KeyPath:    hopSrv.IdentityFile,
			})
		}
	}

	// Destination hop
	pwd, _ := h.vlt.Get(srv.User + "@" + srv.Host)
	hops = append(hops, internalssh.HopConfig{
		Host:       h.resolveHost(ctx, srv.Host),
		Port:       srv.EffectivePort(),
		User:       srv.User,
		Password:   pwd,
		AuthMethod: srv.AuthMethod,
		KeyPath:    srv.IdentityFile,
	})
	return hops, nil
}

// wsRunner adapts *internalssh.Client to plugin.RemoteRunner for web terminal sessions.
// RunInteractive is not used in this path (PTY is managed by the WebSocket handler directly).
type wsRunner struct {
	client *internalssh.Client
}

func (r *wsRunner) Run(_ context.Context, cmd string) (string, int, error) {
	out, err := r.client.Run(cmd)
	if err != nil {
		if exitErr, ok := err.(*gossh.ExitError); ok {
			return string(out), exitErr.ExitStatus(), nil
		}
		return string(out), -1, err
	}
	return string(out), 0, nil
}

func (r *wsRunner) RunPTY(_ context.Context, cmd string, rules []plugin.PTYRule) (string, error) {
	sess, err := r.client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	if err := sess.RequestPty("xterm-256color", 24, 80, gossh.TerminalModes{
		gossh.ECHO: 1, gossh.TTY_OP_ISPEED: 14400, gossh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		return "", err
	}
	stdinPipe, err := sess.StdinPipe()
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	var mu sync.Mutex
	sess.Stdout = &expectWriter{buf: &buf, mu: &mu, stdin: stdinPipe, rules: rules}
	sess.Stderr = sess.Stdout
	if err := sess.Start(cmd); err != nil {
		return "", err
	}
	_ = sess.Wait()
	return buf.String(), nil
}

func (r *wsRunner) RunInteractive(_ context.Context, _ string, _ []plugin.PTYRule, _ map[string]string) error {
	return nil // not used: web handler manages the PTY directly
}

type expectWriter struct {
	buf   *bytes.Buffer
	mu    *sync.Mutex
	stdin io.Writer
	rules []plugin.PTYRule
}

func (w *expectWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	combined := w.buf.String()
	for i := range w.rules {
		r := &w.rules[i]
		if r.Pattern != "" && strings.Contains(combined, r.Pattern) {
			send := r.Send
			if r.SendNewline {
				send += "\n"
			}
			_, _ = w.stdin.Write([]byte(send))
			r.Pattern = ""
		}
	}
	return n, err
}

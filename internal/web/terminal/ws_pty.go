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

	autoGW := r.URL.Query().Get("auto_gw") == "true"
	appName := r.URL.Query().Get("app")

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	if err := h.runSession(r.Context(), ws, serverID, autoGW, appName); err != nil {
		msg, _ := json.Marshal(wsMessage{Type: "data", Data: "\r\nError: " + err.Error() + "\r\n"})
		ws.WriteMessage(websocket.TextMessage, msg)
	}
}

func (h *Handler) runSession(ctx context.Context, ws *websocket.Conn, serverID int64, autoGW bool, appName string) error {
	srv, err := h.db.Servers.GetByID(ctx, serverID)
	if err != nil || srv == nil {
		return fmt.Errorf("server %d not found", serverID)
	}

	hops, err := h.buildHops(ctx, srv, autoGW)
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
		if loadErr == nil && p != nil && p.Runtime.Environments.Native != nil {
			strategy := &plugin.Strategy{
				Kind:    "native",
				Command: p.Runtime.Environments.Native.Command,
				Args:    p.Runtime.Environments.Native.Args,
			}
			if strategy.Command != "" {
				// Resolve secrets and build PTY automation rules.
				if secrets, sErr := plugin.ResolveSecrets(p, h.vlt); sErr == nil {
					sess := &plugin.Session{Plugin: p, Strategy: strategy, Secrets: secrets}
					ptyRules = sess.BuildPTYRules()
				}
				cmd := strategy.BuildCommand()
				if err := sess.Start(cmd); err != nil {
					return fmt.Errorf("start plugin %s: %w", appName, err)
				}
				goto sessionStarted
			}
		}
		// Plugin load failed — fall through to shell with a notice
		stdinPipe.Write([]byte("# plugin '" + appName + "' not loaded, opening shell\r\n"))
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

func (h *Handler) buildHops(ctx context.Context, srv *model.Server, autoGW bool) ([]internalssh.HopConfig, error) {
	var hops []internalssh.HopConfig

	if autoGW {
		// Case 1: named gateway route (GatewayID set).
		if srv.GatewayID != nil {
			gwHops, err := h.db.Gateways.HopsFor(ctx, srv.ID)
			if err != nil {
				return nil, err
			}
			for _, gh := range gwHops {
				hopSrv, err := h.db.Servers.GetByID(ctx, gh.ServerID)
				if err != nil || hopSrv == nil {
					continue
				}
				pwd, _ := h.vlt.Get(hopSrv.User + "@" + hopSrv.Host)
				hops = append(hops, internalssh.HopConfig{
					Host:     h.resolveHost(ctx, hopSrv.Host),
					Port:     hopSrv.EffectivePort(),
					User:     hopSrv.User,
					Password: pwd,
				})
			}
		} else if srv.GatewayServerID != nil {
			// Case 2: recursive server chain (GatewayServerID set).
			chain, err := h.resolveGatewayChain(ctx, srv)
			if err != nil {
				return nil, err
			}
			hops = append(hops, chain...)
		}
	}

	// Destination hop
	pwd, _ := h.vlt.Get(srv.User + "@" + srv.Host)
	hops = append(hops, internalssh.HopConfig{
		Host:     h.resolveHost(ctx, srv.Host),
		Port:     srv.EffectivePort(),
		User:     srv.User,
		Password: pwd,
	})
	return hops, nil
}

// resolveGatewayChain follows gateway_server_id links recursively, returning hops
// in order [outermost-gw ... innermost-gw] (destination appended by caller).
func (h *Handler) resolveGatewayChain(ctx context.Context, dest *model.Server) ([]internalssh.HopConfig, error) {
	var chain []internalssh.HopConfig
	visited := map[int64]bool{dest.ID: true}

	cur := dest
	for cur.GatewayServerID != nil {
		gwSrv, err := h.db.Servers.GetByID(ctx, *cur.GatewayServerID)
		if err != nil || gwSrv == nil {
			return nil, fmt.Errorf("gateway server %d not found", *cur.GatewayServerID)
		}
		if visited[gwSrv.ID] {
			return nil, fmt.Errorf("gateway loop detected at server %s", gwSrv.Host)
		}
		visited[gwSrv.ID] = true

		pwd, _ := h.vlt.Get(gwSrv.User + "@" + gwSrv.Host)
		// Prepend so the outermost gateway is first.
		chain = append([]internalssh.HopConfig{{
			Host:     h.resolveHost(ctx, gwSrv.Host),
			Port:     gwSrv.EffectivePort(),
			User:     gwSrv.User,
			Password: pwd,
		}}, chain...)

		cur = gwSrv
	}

	// If the outermost server in the GatewayServerID chain itself has a named
	// gateway route (GatewayID), prepend those hops to complete the full path.
	if cur.GatewayID != nil {
		gwHops, err := h.db.Gateways.HopsFor(ctx, cur.ID)
		if err != nil {
			return nil, fmt.Errorf("gateway hops for %s: %w", cur.Host, err)
		}
		var prefix []internalssh.HopConfig
		for _, hop := range gwHops {
			hopSrv, err := h.db.Servers.GetByID(ctx, hop.ServerID)
			if err != nil || hopSrv == nil {
				return nil, fmt.Errorf("gateway hop server %d not found", hop.ServerID)
			}
			pwd, _ := h.vlt.Get(hopSrv.User + "@" + hopSrv.Host)
			prefix = append(prefix, internalssh.HopConfig{
				Host:     h.resolveHost(ctx, hopSrv.Host),
				Port:     hopSrv.EffectivePort(),
				User:     hopSrv.User,
				Password: pwd,
			})
		}
		chain = append(prefix, chain...)
	}

	return chain, nil
}

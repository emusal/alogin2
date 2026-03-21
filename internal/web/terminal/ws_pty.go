// Package terminal bridges a WebSocket connection to an SSH PTY session.
package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/emusal/alogin2/internal/db"
	"github.com/emusal/alogin2/internal/model"
	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/emusal/alogin2/internal/vault"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	gossh "golang.org/x/crypto/ssh"
)

// Handler handles WebSocket terminal connections.
type Handler struct {
	db  *db.DB
	vlt vault.Vault
}

// NewHandler creates a terminal WebSocket handler.
func NewHandler(database *db.DB, vlt vault.Vault) *Handler {
	return &Handler{db: database, vlt: vlt}
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

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	if err := h.runSession(r.Context(), ws, serverID); err != nil {
		msg, _ := json.Marshal(wsMessage{Type: "data", Data: "\r\nError: " + err.Error() + "\r\n"})
		ws.WriteMessage(websocket.TextMessage, msg)
	}
}

func (h *Handler) runSession(ctx context.Context, ws *websocket.Conn, serverID int64) error {
	srv, err := h.db.Servers.GetByID(ctx, serverID)
	if err != nil || srv == nil {
		return fmt.Errorf("server %d not found", serverID)
	}

	hops, err := h.buildHops(ctx, srv)
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

	if err := sess.Shell(); err != nil {
		return err
	}

	var wg sync.WaitGroup

	// SSH stdout → WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, readErr := stdoutPipe.Read(buf)
			if n > 0 {
				msg, _ := json.Marshal(wsMessage{Type: "data", Data: string(buf[:n])})
				ws.WriteMessage(websocket.TextMessage, msg)
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

func (h *Handler) buildHops(ctx context.Context, srv *model.Server) ([]internalssh.HopConfig, error) {
	var hops []internalssh.HopConfig

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
				Host:     hopSrv.Host,
				Port:     hopSrv.EffectivePort(),
				User:     hopSrv.User,
				Password: pwd,
			})
		}
	}

	pwd, _ := h.vlt.Get(srv.User + "@" + srv.Host)
	hops = append(hops, internalssh.HopConfig{
		Host:     srv.Host,
		Port:     srv.EffectivePort(),
		User:     srv.User,
		Password: pwd,
	})
	return hops, nil
}

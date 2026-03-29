package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"

	"github.com/emusal/alogin2/internal/mcp"
	"github.com/go-chi/chi/v5"
)

type clusterExecRequest struct {
	Command    string  `json:"command"`
	ServerIDs  []int64 `json:"server_ids"` // empty = all members
	AutoGW     bool    `json:"auto_gw"`
	TimeoutSec int     `json:"timeout_sec"`
}

type clusterExecResult struct {
	ServerID int64  `json:"server_id"`
	Host     string `json:"host"`
	User     string `json:"user"`
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error"`
}

func (h *Handler) execCluster(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid cluster id", http.StatusBadRequest)
		return
	}

	var req clusterExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Command == "" {
		jsonError(w, "command is required", http.StatusBadRequest)
		return
	}
	if req.TimeoutSec <= 0 {
		req.TimeoutSec = 30
	}

	cl, err := h.db.Clusters.GetByID(r.Context(), id)
	if err != nil {
		jsonError(w, "failed to load cluster", http.StatusInternalServerError)
		return
	}
	if cl == nil {
		jsonError(w, "cluster not found", http.StatusNotFound)
		return
	}

	// Build the filter set
	filterSet := make(map[int64]bool, len(req.ServerIDs))
	for _, sid := range req.ServerIDs {
		filterSet[sid] = true
	}

	// Collect target members
	type targetMember struct {
		serverID int64
		user     string
	}
	var targets []targetMember
	for _, m := range cl.Members {
		if len(filterSet) > 0 && !filterSet[m.ServerID] {
			continue
		}
		targets = append(targets, targetMember{serverID: m.ServerID, user: m.User})
	}

	results := make([]clusterExecResult, len(targets))

	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func(idx int, tgt targetMember) {
			defer wg.Done()

			// Load server for host/user info
			srv, err := h.db.Servers.GetByID(r.Context(), tgt.serverID)
			if err != nil || srv == nil {
				results[idx] = clusterExecResult{
					ServerID: tgt.serverID,
					Error:    "server not found",
				}
				return
			}

			user := tgt.user
			if user == "" {
				user = srv.User
			}
			results[idx].ServerID = tgt.serverID
			results[idx].Host = srv.Host
			results[idx].User = user

			cmdResults, execErr := mcp.ExecOnServer(r.Context(), h.db, h.vlt, mcp.ExecRequest{
				ServerID:   tgt.serverID,
				Commands:   []string{req.Command},
				AutoGW:     req.AutoGW,
				TimeoutSec: req.TimeoutSec,
			})
			if execErr != nil {
				results[idx].Error = execErr.Error()
				return
			}
			if len(cmdResults) > 0 {
				results[idx].Output = cmdResults[0].Output
				results[idx].ExitCode = cmdResults[0].ExitCode
				results[idx].Error = cmdResults[0].Error
			}
		}(i, t)
	}
	wg.Wait()

	jsonOK(w, results)
}

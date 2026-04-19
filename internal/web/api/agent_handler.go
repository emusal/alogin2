package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/emusal/alogin2/internal/policy"
	"github.com/go-chi/chi/v5"
)

func configDir() string {
	if d := os.Getenv("ALOGIN_CONFIG_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "alogin")
}

func (h *Handler) listPendingApprovals(w http.ResponseWriter, r *http.Request) {
	pending, err := policy.ListPending(configDir())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if pending == nil {
		pending = []*policy.PendingRequest{}
	}
	jsonOK(w, pending)
}

func (h *Handler) approveRequest(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		jsonError(w, "token is required", http.StatusBadRequest)
		return
	}
	if err := policy.Approve(configDir(), token); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "approved", "token": token})
}

func (h *Handler) denyRequest(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		jsonError(w, "token is required", http.StatusBadRequest)
		return
	}
	if err := policy.Deny(configDir(), token); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "denied", "token": token})
}

func (h *Handler) listTrustWindows(w http.ResponseWriter, r *http.Request) {
	windows, err := policy.ListTrust(configDir())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if windows == nil {
		windows = []*policy.TrustWindow{}
	}
	jsonOK(w, windows)
}

func (h *Handler) grantTrust(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Scope    string `json:"scope"`
		Duration string `json:"duration"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Scope == "" {
		jsonError(w, "scope is required", http.StatusBadRequest)
		return
	}
	dur := 30 * time.Minute
	if req.Duration != "" {
		if d, err := time.ParseDuration(req.Duration); err == nil {
			dur = d
		}
	}
	tw, err := policy.GrantTrust(configDir(), req.Scope, dur, "web-ui")
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, tw)
}

func (h *Handler) revokeTrust(w http.ResponseWriter, r *http.Request) {
	scope := chi.URLParam(r, "scope")
	decoded, err := strconv.Unquote(`"` + scope + `"`)
	if err != nil {
		decoded = scope
	}
	if err := policy.RevokeTrust(configDir(), decoded); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "revoked"})
}

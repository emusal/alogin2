// Package api provides the REST API for the alogin Web UI.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/emusal/alogin2/internal/db"
	"github.com/emusal/alogin2/internal/model"
	"github.com/emusal/alogin2/internal/plugin"
	tunnelpkg "github.com/emusal/alogin2/internal/tunnel"
	"github.com/emusal/alogin2/internal/vault"
	"github.com/go-chi/chi/v5"
)

// Handler holds dependencies for all API routes.
type Handler struct {
	db        *db.DB
	vlt       vault.Vault
	binPath   string // path to the running alogin binary (for tunnel start)
	pluginDir string // path to the plugins directory
}

// NewHandler creates an API handler.
func NewHandler(database *db.DB, vlt vault.Vault) *Handler {
	return &Handler{db: database, vlt: vlt}
}

// NewHandlerWithBin creates an API handler with the binary path for tunnel management.
func NewHandlerWithBin(database *db.DB, vlt vault.Vault, binPath string) *Handler {
	return &Handler{db: database, vlt: vlt, binPath: binPath}
}

// WithPluginDir sets the plugin directory for the handler.
func (h *Handler) WithPluginDir(dir string) *Handler {
	h.pluginDir = dir
	return h
}

// Router returns a chi router with all API routes mounted.
func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()

	// Compute
	r.Get("/compute", h.listServers)
	r.Post("/compute", h.createServer)
	r.Get("/compute/{id}", h.getServer)
	r.Put("/compute/{id}", h.updateServer)
	r.Delete("/compute/{id}", h.deleteServer)

	// Gateways
	r.Get("/gateways", h.listGateways)
	r.Post("/gateways", h.createGateway)
	r.Get("/gateways/{id}", h.getGateway)
	r.Put("/gateways/{id}", h.updateGateway)
	r.Delete("/gateways/{id}", h.deleteGateway)

	// Clusters
	r.Get("/clusters", h.listClusters)
	r.Post("/clusters", h.createCluster)
	r.Get("/clusters/{id}", h.getCluster)
	r.Put("/clusters/{id}", h.updateCluster)
	r.Delete("/clusters/{id}", h.deleteCluster)
	r.Post("/clusters/{id}/exec", h.execCluster)

	// Aliases
	r.Get("/aliases", h.listAliases)

	// Plugins (read-only: loaded from pluginDir)
	r.Get("/plugins", h.listPlugins)

	// Local hosts
	r.Get("/hosts", h.listHosts)
	r.Post("/hosts", h.createHost)
	r.Get("/hosts/{id}", h.getHost)
	r.Put("/hosts/{id}", h.updateHost)
	r.Delete("/hosts/{id}", h.deleteHost)

	// Tunnels
	r.Get("/tunnels", h.listTunnels)
	r.Post("/tunnels", h.createTunnel)
	r.Get("/tunnels/{id}", h.getTunnel)
	r.Put("/tunnels/{id}", h.updateTunnel)
	r.Delete("/tunnels/{id}", h.deleteTunnel)
	r.Post("/tunnels/{id}/start", h.startTunnel)
	r.Post("/tunnels/{id}/stop", h.stopTunnel)
	r.Get("/tunnels/{id}/status", h.tunnelStatus)

	// App-servers
	r.Get("/app-servers", h.listAppServers)
	r.Post("/app-servers", h.createAppServer)
	r.Get("/app-servers/{id}", h.getAppServer)
	r.Put("/app-servers/{id}", h.updateAppServer)
	r.Delete("/app-servers/{id}", h.deleteAppServer)
	r.Post("/app-servers/{id}/connect", h.connectAppServer)

	return r
}

// --- Tunnels ---

func (h *Handler) listTunnels(w http.ResponseWriter, r *http.Request) {
	tunnels, err := h.db.Tunnels.ListAll(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tunnels == nil {
		tunnels = []*model.Tunnel{}
	}
	jsonOK(w, tunnels)
}

func (h *Handler) getTunnel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	t, err := h.db.Tunnels.GetByID(r.Context(), id)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if t == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	jsonOK(w, t)
}

func (h *Handler) createTunnel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		ServerID   int64  `json:"server_id"`
		Direction  string `json:"direction"`
		LocalHost  string `json:"local_host"`
		LocalPort  int    `json:"local_port"`
		RemoteHost string `json:"remote_host"`
		RemotePort int    `json:"remote_port"`
		AutoGW     bool   `json:"auto_gw"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.Direction != "L" && req.Direction != "R" {
		jsonError(w, "direction must be L or R", http.StatusBadRequest)
		return
	}
	if req.LocalPort <= 0 || req.RemotePort <= 0 {
		jsonError(w, "local_port and remote_port must be positive", http.StatusBadRequest)
		return
	}
	if req.LocalHost == "" {
		req.LocalHost = "127.0.0.1"
	}
	t := &model.Tunnel{
		Name:       req.Name,
		ServerID:   req.ServerID,
		Direction:  model.TunnelDirection(req.Direction),
		LocalHost:  req.LocalHost,
		LocalPort:  req.LocalPort,
		RemoteHost: req.RemoteHost,
		RemotePort: req.RemotePort,
		AutoGW:     req.AutoGW,
	}
	if err := h.db.Tunnels.Create(r.Context(), t); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			jsonError(w, "tunnel name already exists", http.StatusConflict)
			return
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, t)
}

func (h *Handler) updateTunnel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	existing, err := h.db.Tunnels.GetByID(r.Context(), id)
	if err != nil || existing == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	var req struct {
		Name       string `json:"name"`
		ServerID   int64  `json:"server_id"`
		Direction  string `json:"direction"`
		LocalHost  string `json:"local_host"`
		LocalPort  int    `json:"local_port"`
		RemoteHost string `json:"remote_host"`
		RemotePort int    `json:"remote_port"`
		AutoGW     bool   `json:"auto_gw"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.ServerID > 0 {
		existing.ServerID = req.ServerID
	}
	if req.Direction == "L" || req.Direction == "R" {
		existing.Direction = model.TunnelDirection(req.Direction)
	}
	if req.LocalHost != "" {
		existing.LocalHost = req.LocalHost
	}
	if req.LocalPort > 0 {
		existing.LocalPort = req.LocalPort
	}
	if req.RemoteHost != "" {
		existing.RemoteHost = req.RemoteHost
	}
	if req.RemotePort > 0 {
		existing.RemotePort = req.RemotePort
	}
	existing.AutoGW = req.AutoGW
	if err := h.db.Tunnels.Update(r.Context(), existing); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, existing)
}

func (h *Handler) deleteTunnel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.db.Tunnels.Delete(r.Context(), id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) startTunnel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	t, err := h.db.Tunnels.GetByID(r.Context(), id)
	if err != nil || t == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	if err := tunnelpkg.Start(t.Name, h.binPath); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "started", "session": tunnelpkg.SessionName(t.Name)})
}

func (h *Handler) stopTunnel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	t, err := h.db.Tunnels.GetByID(r.Context(), id)
	if err != nil || t == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	if err := tunnelpkg.Stop(t.Name); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "stopped"})
}

func (h *Handler) tunnelStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	t, err := h.db.Tunnels.GetByID(r.Context(), id)
	if err != nil || t == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	running := tunnelpkg.IsRunning(t.Name)
	jsonOK(w, map[string]any{
		"running": running,
		"session": tunnelpkg.SessionName(t.Name),
	})
}

// --- Servers ---

func (h *Handler) listServers(w http.ResponseWriter, r *http.Request) {
	servers, err := h.db.Servers.ListAll(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, servers)
}

func (h *Handler) getServer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	srv, err := h.db.Servers.GetByID(r.Context(), id)
	if err != nil || srv == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	jsonOK(w, srv)
}

func (h *Handler) createServer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Protocol        string `json:"protocol"`
		Host            string `json:"host"`
		User            string `json:"user"`
		Password        string `json:"password"`
		Port            int    `json:"port"`
		GatewayID       *int64 `json:"gateway_id"`
		GatewayServerID *int64 `json:"gateway_server_id"`
		Locale          string `json:"locale"`
		DeviceType      string `json:"device_type"`
		Note            string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	srv := &model.Server{
		Protocol:        model.Protocol(req.Protocol),
		Host:            req.Host,
		User:            req.User,
		Port:            req.Port,
		GatewayID:       req.GatewayID,
		GatewayServerID: req.GatewayServerID,
		Locale:          req.Locale,
		DeviceType:      model.DeviceType(req.DeviceType),
		Note:            req.Note,
	}
	if err := h.db.Servers.Create(r.Context(), srv, req.Password); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			jsonError(w, "host/user already exists", http.StatusConflict)
			return
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	created, err := h.db.Servers.GetByHost(r.Context(), srv.Host, srv.User)
	if err != nil || created == nil {
		jsonError(w, "server created but could not be fetched", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, created)
}

func (h *Handler) updateServer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	existing, err := h.db.Servers.GetByID(r.Context(), id)
	if err != nil || existing == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	var req struct {
		Protocol        string `json:"protocol"`
		User            string `json:"user"`
		Password        string `json:"password"`
		Port            int    `json:"port"`
		GatewayID       *int64 `json:"gateway_id"`
		GatewayServerID *int64 `json:"gateway_server_id"`
		Locale          string `json:"locale"`
		DeviceType      string `json:"device_type"`
		Note            string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	srv := &model.Server{
		ID:              id,
		Protocol:        model.Protocol(req.Protocol),
		Host:            existing.Host,
		User:            req.User,
		Port:            req.Port,
		GatewayID:       req.GatewayID,
		GatewayServerID: req.GatewayServerID,
		Locale:          req.Locale,
		DeviceType:      model.DeviceType(req.DeviceType),
		Note:            req.Note,
	}
	if err := h.db.Servers.Update(r.Context(), srv, req.Password); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	updated, _ := h.db.Servers.GetByID(r.Context(), id)
	jsonOK(w, updated)
}

func (h *Handler) deleteServer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.db.Servers.Delete(r.Context(), id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Gateways ---

func (h *Handler) listGateways(w http.ResponseWriter, r *http.Request) {
	gws, err := h.db.Gateways.ListAll(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, gws)
}

func (h *Handler) createGateway(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string  `json:"name"`
		HopServerIDs []int64 `json:"hop_server_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	gw, err := h.db.Gateways.Create(r.Context(), req.Name, req.HopServerIDs)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			jsonError(w, "gateway name already exists", http.StatusConflict)
			return
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, gw)
}

func (h *Handler) getGateway(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	gw, err := h.db.Gateways.GetByID(r.Context(), id)
	if err != nil || gw == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	jsonOK(w, gw)
}

func (h *Handler) updateGateway(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	existing, err := h.db.Gateways.GetByID(r.Context(), id)
	if err != nil || existing == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	var req struct {
		Name         string  `json:"name"`
		HopServerIDs []int64 `json:"hop_server_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	gw, err := h.db.Gateways.Update(r.Context(), id, req.Name, req.HopServerIDs)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			jsonError(w, "gateway name already exists", http.StatusConflict)
			return
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, gw)
}

func (h *Handler) deleteGateway(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.db.Gateways.Delete(r.Context(), id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Clusters ---

func (h *Handler) listClusters(w http.ResponseWriter, r *http.Request) {
	clusters, err := h.db.Clusters.ListAll(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, clusters)
}

func (h *Handler) getCluster(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	c, err := h.db.Clusters.GetByID(r.Context(), id)
	if err != nil || c == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	jsonOK(w, c)
}

func (h *Handler) createCluster(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Members []struct {
			ServerID int64  `json:"server_id"`
			User     string `json:"user"`
		} `json:"members"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	members := make([]model.ClusterMember, len(req.Members))
	for i, m := range req.Members {
		members[i] = model.ClusterMember{ServerID: m.ServerID, User: m.User, MemberOrder: i}
	}
	c, err := h.db.Clusters.Create(r.Context(), req.Name, members)
	if err != nil {
		if strings.Contains(err.Error(), "clusters.name") {
			jsonError(w, "cluster name already exists", http.StatusConflict)
			return
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, c)
}

func (h *Handler) updateCluster(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	existing, err := h.db.Clusters.GetByID(r.Context(), id)
	if err != nil || existing == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	var req struct {
		Name    string `json:"name"`
		Members []struct {
			ServerID int64  `json:"server_id"`
			User     string `json:"user"`
		} `json:"members"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	members := make([]model.ClusterMember, len(req.Members))
	for i, m := range req.Members {
		members[i] = model.ClusterMember{ServerID: m.ServerID, User: m.User, MemberOrder: i}
	}
	c, err := h.db.Clusters.Update(r.Context(), id, req.Name, members)
	if err != nil {
		if strings.Contains(err.Error(), "clusters.name") {
			jsonError(w, "cluster name already exists", http.StatusConflict)
			return
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, c)
}

func (h *Handler) deleteCluster(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.db.Clusters.Delete(r.Context(), id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Local Hosts ---

func (h *Handler) listHosts(w http.ResponseWriter, r *http.Request) {
	hosts, err := h.db.Hosts.ListAll(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, hosts)
}

func (h *Handler) getHost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	hosts, err := h.db.Hosts.ListAll(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, host := range hosts {
		if host.ID == id {
			jsonOK(w, host)
			return
		}
	}
	jsonError(w, "not found", http.StatusNotFound)
}

func (h *Handler) createHost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hostname    string `json:"hostname"`
		IP          string `json:"ip"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Hostname == "" || req.IP == "" {
		jsonError(w, "hostname and ip are required", http.StatusBadRequest)
		return
	}
	host := &model.LocalHost{
		Hostname:    req.Hostname,
		IP:          req.IP,
		Description: req.Description,
	}
	if err := h.db.Hosts.Create(r.Context(), host); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			jsonError(w, "hostname already exists", http.StatusConflict)
			return
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, host)
}

func (h *Handler) updateHost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	var req struct {
		IP          string `json:"ip"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.IP == "" {
		jsonError(w, "ip is required", http.StatusBadRequest)
		return
	}
	// Fetch existing to preserve hostname
	hosts, err := h.db.Hosts.ListAll(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var existing *model.LocalHost
	for _, host := range hosts {
		if host.ID == id {
			existing = host
			break
		}
	}
	if existing == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	existing.IP = req.IP
	existing.Description = req.Description
	if err := h.db.Hosts.Update(r.Context(), existing); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, existing)
}

func (h *Handler) deleteHost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.db.Hosts.Delete(r.Context(), id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Plugins ---

func (h *Handler) listPlugins(w http.ResponseWriter, r *http.Request) {
	if h.pluginDir == "" {
		jsonOK(w, []any{})
		return
	}
	plugins, err := plugin.LoadDir(h.pluginDir)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type row struct {
		Name       string   `json:"name"`
		Version    string   `json:"version"`
		Provider   string   `json:"provider"`
		Strategies []string `json:"strategies"`
		CmdFlag    string   `json:"cmd_flag"`
	}
	rows := make([]row, 0, len(plugins))
	for _, p := range plugins {
		flag := p.Runtime.CmdFlag
		if flag == "" {
			flag = "-e"
		}
		rows = append(rows, row{
			Name:       p.Name,
			Version:    p.Version,
			Provider:   string(p.Auth.Provider),
			Strategies: p.Runtime.Strategies,
			CmdFlag:    flag,
		})
	}
	jsonOK(w, rows)
}

// --- Aliases ---

func (h *Handler) listAliases(w http.ResponseWriter, r *http.Request) {
	aliases, err := h.db.Aliases.ListAll(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, aliases)
}

// --- App-servers ---

func (h *Handler) listAppServers(w http.ResponseWriter, r *http.Request) {
	list, err := h.db.AppServers.ListAll(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []*model.AppServer{}
	}
	jsonOK(w, list)
}

func (h *Handler) getAppServer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	as, err := h.db.AppServers.GetByID(r.Context(), id)
	if err != nil || as == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	jsonOK(w, as)
}

func (h *Handler) createAppServer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		ServerID    int64  `json:"server_id"`
		PluginName  string `json:"plugin_name"`
		AutoGW      bool   `json:"auto_gw"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.ServerID == 0 || req.PluginName == "" {
		jsonError(w, "name, server_id, and plugin_name are required", http.StatusBadRequest)
		return
	}
	as := &model.AppServer{
		Name:        req.Name,
		ServerID:    req.ServerID,
		PluginName:  req.PluginName,
		AutoGW:      req.AutoGW,
		Description: req.Description,
	}
	if err := h.db.AppServers.Create(r.Context(), as); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, as)
}

func (h *Handler) updateAppServer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	existing, err := h.db.AppServers.GetByID(r.Context(), id)
	if err != nil || existing == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	var req struct {
		Name        string `json:"name"`
		ServerID    int64  `json:"server_id"`
		PluginName  string `json:"plugin_name"`
		AutoGW      *bool  `json:"auto_gw"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.ServerID > 0 {
		existing.ServerID = req.ServerID
	}
	if req.PluginName != "" {
		existing.PluginName = req.PluginName
	}
	if req.AutoGW != nil {
		existing.AutoGW = *req.AutoGW
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	if err := h.db.AppServers.Update(r.Context(), existing); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, existing)
}

func (h *Handler) deleteAppServer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.db.AppServers.Delete(r.Context(), id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) connectAppServer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	as, err := h.db.AppServers.GetByID(r.Context(), id)
	if err != nil || as == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	jsonOK(w, map[string]any{
		"server_id": as.ServerID,
		"auto_gw":   as.AutoGW,
		"app":       as.PluginName,
	})
}

// --- helpers ---

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/emusal/alogin2/internal/model"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) listProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.db.Profiles.ListAll(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if profiles == nil {
		profiles = []*model.Profile{}
	}
	jsonOK(w, profiles)
}

func (h *Handler) getProfile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	p, err := h.db.Profiles.GetByID(r.Context(), id)
	if err != nil || p == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	jsonOK(w, p)
}

func (h *Handler) createProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string `json:"name"`
		Description    string `json:"description"`
		GatewayRouteID *int64 `json:"gateway_route_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	p, err := h.db.Profiles.Create(r.Context(), req.Name, req.Description, req.GatewayRouteID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			jsonError(w, "profile name already exists", http.StatusConflict)
			return
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, p)
}

func (h *Handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	existing, err := h.db.Profiles.GetByID(r.Context(), id)
	if err != nil || existing == nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	var req struct {
		Name           string `json:"name"`
		Description    string `json:"description"`
		GatewayRouteID *int64 `json:"gateway_route_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	existing.Description = req.Description
	routeID := req.GatewayRouteID
	if routeID == nil {
		routeID = existing.GatewayRouteID
	}
	if err := h.db.Profiles.Update(r.Context(), existing, routeID); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			jsonError(w, "profile name already exists", http.StatusConflict)
			return
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	updated, _ := h.db.Profiles.GetByID(r.Context(), id)
	jsonOK(w, updated)
}

func (h *Handler) deleteProfile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.db.Profiles.Delete(r.Context(), id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) activateProfile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.db.Profiles.SetActive(r.Context(), id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p, _ := h.db.Profiles.GetByID(r.Context(), id)
	jsonOK(w, p)
}

func (h *Handler) deactivateProfile(w http.ResponseWriter, r *http.Request) {
	if err := h.db.Profiles.SetActive(r.Context(), 0); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

package handlers

import (
	"clouddrive/middleware"
	"clouddrive/services"
	"encoding/json"
	"net/http"
)

type PermissionsHandler struct {
	permStore *services.PermissionStore
}

func NewPermissionsHandler(permStore *services.PermissionStore) *PermissionsHandler {
	return &PermissionsHandler{permStore: permStore}
}

// SetPrivate marks a folder as private
func (h *PermissionsHandler) SetPrivate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path         string   `json:"path"`
		AllowedUsers []string `json:"allowedUsers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	username := middleware.GetUsername(r)

	// If no allowed users specified, default to just the current user
	if len(req.AllowedUsers) == 0 {
		req.AllowedUsers = []string{username}
	}

	if err := h.permStore.SetPrivate(req.Path, username, req.AllowedUsers); err != nil {
		http.Error(w, "Failed to set permissions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "private", "path": req.Path})
}

// RemovePrivate removes the restriction from a folder
func (h *PermissionsHandler) RemovePrivate(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	username := middleware.GetUsername(r)
	role := middleware.GetRole(r)

	// Only owner or admin can remove
	perm := h.permStore.GetPermission(path)
	if perm != nil && perm.Owner != username && role != "admin" {
		http.Error(w, "Only the owner or admin can remove restrictions", http.StatusForbidden)
		return
	}

	if err := h.permStore.RemovePrivate(path); err != nil {
		http.Error(w, "Failed to remove permissions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "public", "path": path})
}

// GetPermission returns the permission info for a path
func (h *PermissionsHandler) GetPermission(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	perm := h.permStore.GetPermission(path)
	if perm == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"isPrivate": false, "path": path})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"isPrivate":    true,
		"path":         path,
		"owner":        perm.Owner,
		"allowedUsers": perm.AllowedUsers,
	})
}

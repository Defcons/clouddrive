package handlers

import (
	"clouddrive/middleware"
	"clouddrive/services"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

type PermissionsHandler struct {
	permStore *services.PermissionStore
	audit     *services.AuditLogger
}

func NewPermissionsHandler(permStore *services.PermissionStore, audit *services.AuditLogger) *PermissionsHandler {
	return &PermissionsHandler{permStore: permStore, audit: audit}
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
	role := middleware.GetRole(r)

	// Authz: non-admins may only manage permissions on paths inside their own
	// home folder, and may never override an entry owned by someone else.
	if role != "admin" {
		home := middleware.GetHomeFolder(r)
		if home == "" || home == "/" {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
		cp := filepath.ToSlash(filepath.Clean(req.Path))
		ch := filepath.ToSlash(filepath.Clean(home))
		if cp != ch && !strings.HasPrefix(cp, ch+"/") {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
	}
	if existing := h.permStore.GetPermission(req.Path); existing != nil && existing.Owner != username && role != "admin" {
		http.Error(w, "Only the owner or admin can change these permissions", http.StatusForbidden)
		return
	}

	// If no allowed users specified, default to just the current user
	if len(req.AllowedUsers) == 0 {
		req.AllowedUsers = []string{username}
	}

	if err := h.permStore.SetPrivate(req.Path, username, req.AllowedUsers); err != nil {
		http.Error(w, "Failed to set permissions", http.StatusInternalServerError)
		return
	}

	if h.audit != nil {
		h.audit.Log("PRIVATE", middleware.GetUsername(r), getClientIP(r), fmt.Sprintf("made %s private", req.Path))
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

	if h.audit != nil {
		h.audit.Log("PUBLIC", middleware.GetUsername(r), getClientIP(r), fmt.Sprintf("made %s public", path))
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

	// Don't leak the owner/ACL of a path the caller can't access; report it as
	// non-private (same non-disclosure pattern as GetTags).
	if !userCanAccess(r, h.permStore, path) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"isPrivate": false, "path": path})
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

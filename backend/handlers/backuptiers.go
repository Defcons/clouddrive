package handlers

import (
	"clouddrive/middleware"
	"clouddrive/services"
	"encoding/json"
	"fmt"
	"net/http"
)

type BackupTierHandler struct {
	store     *services.BackupTierStore
	permStore *services.PermissionStore
	audit     *services.AuditLogger
	settings  *services.SettingsStore // optional; gates the offsite feature
}

func NewBackupTierHandler(store *services.BackupTierStore, permStore *services.PermissionStore, audit *services.AuditLogger) *BackupTierHandler {
	return &BackupTierHandler{store: store, permStore: permStore, audit: audit}
}

// SetSettings wires the instance settings so the offsite-backup feature can be
// disabled instance-wide.
func (h *BackupTierHandler) SetSettings(s *services.SettingsStore) { h.settings = s }

// Get the tier for a path (with inheritance from parents)
func (h *BackupTierHandler) Get(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	if !userCanAccess(r, h.permStore, path) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	tier := h.store.GetTier(path)
	exact := h.store.GetTierExact(path)
	inherited := tier != exact

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":      path,
		"tier":      tier,
		"exact":     exact,
		"inherited": inherited,
	})
}

// Set the tier for a path. tier 0 removes the entry.
func (h *BackupTierHandler) Set(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
		Tier int    `json:"tier"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Tier != 0 && req.Tier != 2 {
		http.Error(w, "tier must be 0 (none) or 2 (offsite)", http.StatusBadRequest)
		return
	}

	// Flagging a folder for offsite backup is gated by the instance setting.
	// Tier 0 (clearing an existing flag) is always allowed so admins can turn
	// the feature off and let users un-flag folders.
	if req.Tier == 2 && h.settings != nil && !h.settings.Get().OffsiteBackupEnabled {
		http.Error(w, "Offsite backup is disabled on this instance", http.StatusForbidden)
		return
	}

	if !userCanAccess(r, h.permStore, req.Path) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	if err := h.store.SetTier(req.Path, req.Tier); err != nil {
		http.Error(w, "Failed to save", http.StatusInternalServerError)
		return
	}

	if h.audit != nil {
		action := "BACKUP_TIER_CLEAR"
		detail := fmt.Sprintf("removed backup tier for %s", req.Path)
		if req.Tier == 2 {
			action = "BACKUP_TIER_OFFSITE"
			detail = fmt.Sprintf("marked %s for offsite backup", req.Path)
		}
		h.audit.Log(action, middleware.GetUsername(r), getClientIP(r), detail)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"path": req.Path, "tier": req.Tier})
}

// List all tier entries (admin overview). Restricted to admins — the full map
// would otherwise leak the paths of every tiered folder to any user.
func (h *BackupTierHandler) List(w http.ResponseWriter, r *http.Request) {
	if middleware.GetRole(r) != "admin" {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.store.All())
}

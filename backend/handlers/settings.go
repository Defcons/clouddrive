package handlers

import (
	"clouddrive/middleware"
	"clouddrive/services"
	"encoding/json"
	"net/http"
)

type SettingsHandler struct {
	store *services.SettingsStore
	audit *services.AuditLogger
}

func NewSettingsHandler(store *services.SettingsStore, audit *services.AuditLogger) *SettingsHandler {
	return &SettingsHandler{store: store, audit: audit}
}

// Get returns the instance settings. Readable by any authenticated user — the
// frontend needs the instance name + feature flags to render correctly. The
// fields are non-sensitive by design.
func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.store.Get())
}

// Update replaces the instance settings. Admin-only (re-checked here even though
// the route is CSRF/auth-wrapped, so a wiring mistake can't open it up).
func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	if middleware.GetRole(r) != "admin" {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}
	var next services.Settings
	if err := decodeJSON(w, r, &next); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if err := h.store.Update(next); err != nil {
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}
	if h.audit != nil {
		h.audit.Log("SETTINGS_UPDATE", middleware.GetUsername(r), getClientIP(r), "instance settings changed")
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.store.Get())
}

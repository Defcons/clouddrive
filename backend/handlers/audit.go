package handlers

import (
	"clouddrive/middleware"
	"clouddrive/services"
	"encoding/json"
	"net/http"
	"strconv"
)

type AuditHandler struct {
	audit *services.AuditLogger
}

func NewAuditHandler(audit *services.AuditLogger) *AuditHandler {
	return &AuditHandler{audit: audit}
}

func (h *AuditHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	// Admin only
	role := middleware.GetRole(r)
	if role != "admin" {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
			if limit > 1000 {
				limit = 1000
			}
		}
	}

	entries := h.audit.GetRecent(limit)
	if entries == nil {
		entries = []services.AuditEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

package handlers

import (
	"clouddrive/middleware"
	"clouddrive/services"
	"encoding/json"
	"net/http"
)

type TrashHandler struct {
	trash *services.TrashStore
	audit *services.AuditLogger
}

func NewTrashHandler(trash *services.TrashStore, audit *services.AuditLogger) *TrashHandler {
	return &TrashHandler{trash: trash, audit: audit}
}

func (h *TrashHandler) List(w http.ResponseWriter, r *http.Request) {
	username := middleware.GetUsername(r)
	role := middleware.GetRole(r)
	items := h.trash.List(username, role)
	if items == nil {
		items = []services.TrashItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func (h *TrashHandler) Restore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	username := middleware.GetUsername(r)
	role := middleware.GetRole(r)

	if err := h.trash.Restore(req.ID, username, role); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if h.audit != nil {
		h.audit.Log("RESTORE", username, getClientIP(r), "restored from trash: "+req.ID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"restored": req.ID})
}

func (h *TrashHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	username := middleware.GetUsername(r)
	role := middleware.GetRole(r)

	if err := h.trash.PermanentDelete(id, username, role); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"deleted": id})
}

func (h *TrashHandler) Empty(w http.ResponseWriter, r *http.Request) {
	username := middleware.GetUsername(r)
	role := middleware.GetRole(r)

	if err := h.trash.EmptyTrash(username, role); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.audit != nil {
		h.audit.Log("EMPTY_TRASH", username, getClientIP(r), "emptied trash")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "emptied"})
}

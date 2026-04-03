package handlers

import (
	"clouddrive/middleware"
	"clouddrive/services"
	"encoding/json"
	"net/http"
)

type NotificationHandler struct {
	store *services.NotificationStore
}

func NewNotificationHandler(store *services.NotificationStore) *NotificationHandler {
	return &NotificationHandler{store: store}
}

func (h *NotificationHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	username := middleware.GetUsername(r)
	items := h.store.GetAll(username, 50)
	if items == nil {
		items = []services.Notification{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func (h *NotificationHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	username := middleware.GetUsername(r)
	items := h.store.GetUnread(username)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"count": len(items)})
}

func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
		All bool     `json:"all"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	username := middleware.GetUsername(r)

	if req.All {
		h.store.MarkAllRead(username)
	} else {
		h.store.MarkRead(username, req.IDs)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

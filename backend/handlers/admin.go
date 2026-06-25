package handlers

import (
	"clouddrive/middleware"
	"clouddrive/services"
	"encoding/json"
	"net/http"
)

// AdminHandler exposes admin-only user management. Every handler re-checks the
// caller's role — the routes are auth/CSRF wrapped, but authorization lives
// here so it can't be bypassed by a wiring mistake.
type AdminHandler struct {
	users *services.UserStore
	audit *services.AuditLogger
}

func NewAdminHandler(users *services.UserStore, audit *services.AuditLogger) *AdminHandler {
	return &AdminHandler{users: users, audit: audit}
}

func (h *AdminHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if middleware.GetRole(r) != "admin" {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return false
	}
	return true
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.users.ListUsers())
}

func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var req struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		HomeFolder string `json:"homeFolder"`
		Role       string `json:"role"`
		Quota      int64  `json:"quota"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if err := h.users.CreateUser(req.Username, req.Password, req.HomeFolder, req.Role, req.Quota); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if h.audit != nil {
		h.audit.Log("ADMIN_USER_CREATE", middleware.GetUsername(r), getClientIP(r), "created user "+req.Username)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"created": req.Username})
}

func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var req struct {
		Username    string `json:"username"`
		HomeFolder  string `json:"homeFolder"`
		Role        string `json:"role"`
		Quota       int64  `json:"quota"`
		NewPassword string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if err := h.users.UpdateUser(req.Username, req.HomeFolder, req.Role, req.Quota, req.NewPassword); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if h.audit != nil {
		h.audit.Log("ADMIN_USER_UPDATE", middleware.GetUsername(r), getClientIP(r), "updated user "+req.Username)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"updated": req.Username})
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "username required", http.StatusBadRequest)
		return
	}
	// Don't let an admin delete their own account out from under themselves.
	if username == middleware.GetUsername(r) {
		http.Error(w, "You cannot delete your own account", http.StatusBadRequest)
		return
	}
	if err := h.users.DeleteUser(username); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if h.audit != nil {
		h.audit.Log("ADMIN_USER_DELETE", middleware.GetUsername(r), getClientIP(r), "deleted user "+username)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"deleted": username})
}

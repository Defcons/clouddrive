package handlers

import (
	"clouddrive/middleware"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"
)

// Versions lists the stored previous versions of a file.
func (h *FileHandler) Versions(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	if !h.checkAccess(r, filepath.ToSlash(filepath.Dir(filePath))) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if h.versions == nil {
		_ = json.NewEncoder(w).Encode([]any{})
		return
	}
	list := h.versions.ListVersions(filePath)
	if list == nil {
		_ = json.NewEncoder(w).Encode([]any{})
		return
	}
	_ = json.NewEncoder(w).Encode(list)
}

// DownloadVersion streams a specific stored version of a file.
func (h *FileHandler) DownloadVersion(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	id := r.URL.Query().Get("id")
	if filePath == "" || id == "" {
		http.Error(w, "path and id required", http.StatusBadRequest)
		return
	}
	if !h.checkAccess(r, filepath.ToSlash(filepath.Dir(filePath))) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	if h.versions == nil {
		http.Error(w, "Versioning not enabled", http.StatusNotFound)
		return
	}
	f, _, savedNs, err := h.versions.OpenVersion(filePath, id)
	if err != nil {
		http.Error(w, "Version not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(filePath)))
	http.ServeContent(w, r, filepath.Base(filePath), time.Unix(0, savedNs), f)
}

// RestoreVersion restores a stored version over the current file.
func (h *FileHandler) RestoreVersion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
		ID   string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if req.Path == "" || req.ID == "" {
		http.Error(w, "path and id required", http.StatusBadRequest)
		return
	}
	if !h.checkAccess(r, filepath.ToSlash(filepath.Dir(req.Path))) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	if h.versions == nil {
		http.Error(w, "Versioning not enabled", http.StatusNotFound)
		return
	}
	absPath, err := h.safePath(req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.versions.RestoreVersion(req.Path, req.ID, absPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if h.audit != nil {
		h.audit.Log("RESTORE_VERSION", middleware.GetUsername(r), getClientIP(r), fmt.Sprintf("restored version %s of %s", req.ID, req.Path))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"restored": req.Path})
}

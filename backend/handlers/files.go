package handlers

import (
	"clouddrive/middleware"
	"clouddrive/services"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FileHandler struct {
	root      string
	permStore *services.PermissionStore
}

type FileInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDir     bool   `json:"isDir"`
	Size      int64  `json:"size"`
	ModTime   int64  `json:"modTime"`
	ItemCount *int   `json:"itemCount,omitempty"`
	IsPrivate bool   `json:"isPrivate,omitempty"`
}

func NewFileHandler(root string, permStore ...*services.PermissionStore) *FileHandler {
	h := &FileHandler{root: root}
	if len(permStore) > 0 {
		h.permStore = permStore[0]
	}
	return h
}

// safePath resolves and validates a path is within the storage root.
func (h *FileHandler) safePath(reqPath string) (string, error) {
	if reqPath == "" {
		reqPath = "/"
	}
	cleaned := filepath.Clean(reqPath)
	full := filepath.Join(h.root, cleaned)
	abs, err := filepath.Abs(full)
	if err != nil {
		return "", fmt.Errorf("invalid path")
	}
	rootAbs, err := filepath.Abs(h.root)
	if err != nil {
		return "", fmt.Errorf("invalid root")
	}
	if !strings.HasPrefix(abs, rootAbs) {
		return "", fmt.Errorf("path traversal denied")
	}
	return abs, nil
}

// checkAccess verifies the user can access the given path
func (h *FileHandler) checkAccess(r *http.Request, filePath string) bool {
	if h.permStore == nil {
		return true
	}
	username := middleware.GetUsername(r)
	role := middleware.GetRole(r)
	return h.permStore.CanAccess(filePath, username, role)
}

func (h *FileHandler) List(w http.ResponseWriter, r *http.Request) {
	dirPath := r.URL.Query().Get("path")
	if dirPath == "" {
		dirPath = "/"
	}

	// Check if user can access this directory
	if !h.checkAccess(r, dirPath) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	absPath, err := h.safePath(dirPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		http.Error(w, "Cannot read directory", http.StatusNotFound)
		return
	}

	username := middleware.GetUsername(r)
	role := middleware.GetRole(r)

	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		// Skip hidden files/dirs starting with dot
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		entryPath := filepath.Join(dirPath, entry.Name())
		entryPath = filepath.ToSlash(entryPath)

		// Filter out directories the user can't access
		if entry.IsDir() && h.permStore != nil {
			if !h.permStore.CanAccess(entryPath, username, role) {
				continue
			}
		}

		fi := FileInfo{
			Name:    entry.Name(),
			Path:    entryPath,
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().UnixMilli(),
		}
		if entry.IsDir() {
			childEntries, err := os.ReadDir(filepath.Join(absPath, entry.Name()))
			if err == nil {
				count := 0
				for _, ce := range childEntries {
					if !strings.HasPrefix(ce.Name(), ".") {
						count++
					}
				}
				fi.ItemCount = &count
			}
			if h.permStore != nil {
				fi.IsPrivate = h.permStore.IsPrivate(entryPath)
			}
		}
		files = append(files, fi)
	}

	// Sort: directories first, then alphabetically
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func (h *FileHandler) Download(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	if !h.checkAccess(r, filepath.ToSlash(filepath.Dir(filePath))) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	absPath, err := h.safePath(filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	ext := filepath.Ext(absPath)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, info.Name()))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

	f, err := os.Open(absPath)
	if err != nil {
		http.Error(w, "Cannot open file", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	io.Copy(w, f)
}

func (h *FileHandler) Preview(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	if !h.checkAccess(r, filepath.ToSlash(filepath.Dir(filePath))) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	absPath, err := h.safePath(filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	ext := filepath.Ext(absPath)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, info.Name()))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	w.Header().Set("Cache-Control", "private, max-age=3600")

	f, err := os.Open(absPath)
	if err != nil {
		http.Error(w, "Cannot open file", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	io.Copy(w, f)
}

func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	targetDir := r.URL.Query().Get("path")
	if targetDir == "" {
		targetDir = "/"
	}

	if !h.checkAccess(r, targetDir) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	absDir, err := h.safePath(targetDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 500MB max
	r.ParseMultipartForm(500 << 20)

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "No files provided", http.StatusBadRequest)
		return
	}

	uploaded := make([]string, 0, len(files))
	for _, fh := range files {
		src, err := fh.Open()
		if err != nil {
			continue
		}

		dstPath := filepath.Join(absDir, filepath.Base(fh.Filename))
		dst, err := os.Create(dstPath)
		if err != nil {
			src.Close()
			continue
		}

		io.Copy(dst, src)
		src.Close()
		dst.Close()
		uploaded = append(uploaded, fh.Filename)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"uploaded": uploaded,
		"count":    len(uploaded),
	})
}

func (h *FileHandler) Mkdir(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	parentPath := req.Path
	if parentPath == "" {
		parentPath = "/"
	}

	if !h.checkAccess(r, parentPath) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	absParent, err := h.safePath(parentPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	newDir := filepath.Join(absParent, filepath.Base(req.Name))
	if err := os.MkdirAll(newDir, 0755); err != nil {
		http.Error(w, "Failed to create directory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"created": req.Name})
}

func (h *FileHandler) Rename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OldPath string `json:"oldPath"`
		NewName string `json:"newName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if !h.checkAccess(r, filepath.ToSlash(filepath.Dir(req.OldPath))) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	absOld, err := h.safePath(req.OldPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	newPath := filepath.Join(filepath.Dir(absOld), filepath.Base(req.NewName))
	// Verify new path is still within root
	if _, err := h.safePath(filepath.Join(filepath.Dir(req.OldPath), req.NewName)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := os.Rename(absOld, newPath); err != nil {
		http.Error(w, "Failed to rename", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"renamed": req.NewName})
}

func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	if !h.checkAccess(r, filepath.ToSlash(filepath.Dir(filePath))) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	absPath, err := h.safePath(filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Don't allow deleting the root
	rootAbs, _ := filepath.Abs(h.root)
	if absPath == rootAbs {
		http.Error(w, "Cannot delete root", http.StatusForbidden)
		return
	}

	if err := os.RemoveAll(absPath); err != nil {
		http.Error(w, "Failed to delete", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"deleted": filePath})
}

package handlers

import (
	"archive/zip"
	"clouddrive/middleware"
	"clouddrive/services"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

type FileHandler struct {
	root       string
	permStore  *services.PermissionStore
	audit      *services.AuditLogger
	trash      *services.TrashStore
	tags       *services.TagStore
	tierStore  *services.BackupTierStore
}

type FileInfo struct {
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	IsDir      bool     `json:"isDir"`
	Size       int64    `json:"size"`
	CreatedAt  int64    `json:"createdAt"`
	ModTime    int64    `json:"modTime"`
	ItemCount  *int     `json:"itemCount,omitempty"`
	IsPrivate  bool     `json:"isPrivate,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	BackupTier int      `json:"backupTier,omitempty"`
}

// getCreationTime tries to get the file creation/change time, falls back to ModTime
func getCreationTime(info os.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		// Ctim is the status change time on Linux (closest to creation time available)
		ctim := stat.Ctim
		ms := ctim.Sec*1000 + ctim.Nsec/1000000
		if ms > 0 {
			return ms
		}
	}
	return info.ModTime().UnixMilli()
}

func NewFileHandler(root string, permStore *services.PermissionStore, audit *services.AuditLogger, trash *services.TrashStore, tags *services.TagStore, tierStore *services.BackupTierStore) *FileHandler {
	return &FileHandler{root: root, permStore: permStore, audit: audit, trash: trash, tags: tags, tierStore: tierStore}
}

// getClientIP extracts the client IP, preferring the left-most
// X-Forwarded-For value (closest to the real client). Returns raw RemoteAddr
// if no proxy headers are set.
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if comma := strings.Index(xff, ","); comma >= 0 {
			return strings.TrimSpace(xff[:comma])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}

// safePath resolves reqPath and ensures it stays within the storage root.
// It also resolves symlinks — if a link points outside the root, the path
// is rejected. This is critical for preventing zip/download endpoints from
// being used to exfiltrate files via symlinks placed in user folders.
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
	// EvalSymlinks only works if the path exists; ignore error so we can
	// validate parent paths for upload/mkdir scenarios.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	// Use separator to avoid /data/evil being accepted for root /data.
	if abs != rootAbs && !strings.HasPrefix(abs+string(filepath.Separator), rootAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal denied")
	}
	return abs, nil
}

// maxUploadBytes returns the hard request-body cap for uploads (default 5 GiB),
// overridable via MAX_UPLOAD_BYTES (in bytes).
func maxUploadBytes() int64 {
	if v := os.Getenv("MAX_UPLOAD_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return 5 << 30
}

// checkAccess verifies the user can access the given path.
// Non-admin users are restricted to their home folder.
func (h *FileHandler) checkAccess(r *http.Request, filePath string) bool {
	username := middleware.GetUsername(r)
	role := middleware.GetRole(r)

	// Enforce home folder restriction for non-admin users
	if role != "admin" {
		homeFolder := middleware.GetHomeFolder(r)
		if homeFolder != "" && homeFolder != "/" {
			cleanPath := filepath.ToSlash(filepath.Clean(filePath))
			cleanHome := filepath.ToSlash(filepath.Clean(homeFolder))
			if cleanPath != cleanHome && !strings.HasPrefix(cleanPath, cleanHome+"/") {
				return false
			}
		}
	}

	if h.permStore == nil {
		return true
	}
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
			Name:      entry.Name(),
			Path:      entryPath,
			IsDir:     entry.IsDir(),
			Size:      info.Size(),
			CreatedAt: getCreationTime(info),
			ModTime:   info.ModTime().UnixMilli(),
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
		if h.tags != nil {
			if t := h.tags.GetTags(entryPath); len(t) > 0 {
				fi.Tags = t
			}
		}
		if h.tierStore != nil && entry.IsDir() {
			if tier := h.tierStore.GetTier(entryPath); tier > 0 {
				fi.BackupTier = tier
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
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	if info.IsDir() {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, info.Name()))

		zw := zip.NewWriter(w)
		defer zw.Close()

		walkFilesNoSymlinks(absPath, func(fpath string, finfo os.FileInfo, relPath string) {
			header, err := zip.FileInfoHeader(finfo)
			if err != nil {
				return
			}
			header.Name = filepath.ToSlash(filepath.Join(info.Name(), relPath))
			header.Method = zip.Deflate
			writer, err := zw.CreateHeader(header)
			if err != nil {
				return
			}
			file, err := os.Open(fpath)
			if err != nil {
				return
			}
			defer file.Close()
			if _, err := io.Copy(writer, file); err != nil {
				slog.Debug("zip copy failed", "err", err)
			}
		})
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

	// Hard-cap the whole request body (not just the in-memory threshold) so an
	// authenticated user can't fill the disk. Tunable via MAX_UPLOAD_BYTES.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes())
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "Upload too large or malformed", http.StatusRequestEntityTooLarge)
		return
	}

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

		name := filepath.Base(fh.Filename)
		if name == "" || name == "." || name == ".." || strings.HasPrefix(name, ".") {
			src.Close()
			continue // refuse dotfiles — would clobber app state (.permissions.json, etc.)
		}
		dstPath := filepath.Join(absDir, name)
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

	if h.audit != nil && len(uploaded) > 0 {
		h.audit.Log("UPLOAD", middleware.GetUsername(r), getClientIP(r), fmt.Sprintf("uploaded %d file(s) to %s: %s", len(uploaded), targetDir, strings.Join(uploaded, ", ")))
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

	if h.audit != nil {
		h.audit.Log("MKDIR", middleware.GetUsername(r), getClientIP(r), fmt.Sprintf("created folder %s in %s", req.Name, parentPath))
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

	if h.audit != nil {
		h.audit.Log("RENAME", middleware.GetUsername(r), getClientIP(r), fmt.Sprintf("renamed %s to %s", req.OldPath, req.NewName))
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

	username := middleware.GetUsername(r)

	if h.trash != nil {
		if err := h.trash.MoveToTrash(absPath, filePath, username); err != nil {
			http.Error(w, "Failed to move to trash", http.StatusInternalServerError)
			return
		}
	} else {
		if err := os.RemoveAll(absPath); err != nil {
			http.Error(w, "Failed to delete", http.StatusInternalServerError)
			return
		}
	}

	if h.audit != nil {
		h.audit.Log("DELETE", username, getClientIP(r), fmt.Sprintf("deleted %s", filePath))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"deleted": filePath})
}

func (h *FileHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(r.URL.Query().Get("q"))
	if query == "" {
		http.Error(w, "query required", http.StatusBadRequest)
		return
	}

	username := middleware.GetUsername(r)
	role := middleware.GetRole(r)
	homeFolder := middleware.GetHomeFolder(r)
	if homeFolder == "" {
		homeFolder = "/"
	}

	searchRoot := filepath.Join(h.root, homeFolder)
	if role == "admin" {
		searchRoot = h.root
	}

	var results []FileInfo
	_ = filepath.Walk(searchRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || len(results) >= 100 {
			return nil
		}
		// Skip symlinks — don't leak files outside the root via search.
		if info.Mode()&os.ModeSymlink != 0 {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.Contains(strings.ToLower(info.Name()), query) {
			relPath, _ := filepath.Rel(h.root, path)
			entryPath := "/" + filepath.ToSlash(relPath)

			if h.permStore != nil && !h.permStore.CanAccess(entryPath, username, role) {
				return nil
			}

			fi := FileInfo{
				Name:      info.Name(),
				Path:      entryPath,
				IsDir:     info.IsDir(),
				Size:      info.Size(),
				CreatedAt: getCreationTime(info),
				ModTime:   info.ModTime().UnixMilli(),
			}
			results = append(results, fi)
		}
		return nil
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (h *FileHandler) Move(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths       []string `json:"paths"`
		Destination string   `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if !h.checkAccess(r, req.Destination) {
		http.Error(w, "Access denied to destination", http.StatusForbidden)
		return
	}

	absDest, err := h.safePath(req.Destination)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	moved := 0
	for _, p := range req.Paths {
		if !h.checkAccess(r, filepath.ToSlash(filepath.Dir(p))) {
			continue
		}
		absSrc, err := h.safePath(p)
		if err != nil {
			continue
		}
		newPath := filepath.Join(absDest, filepath.Base(absSrc))
		if err := os.Rename(absSrc, newPath); err == nil {
			moved++
		}
	}

	if h.audit != nil {
		h.audit.Log("MOVE", middleware.GetUsername(r), getClientIP(r), fmt.Sprintf("moved %d item(s) to %s", moved, req.Destination))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"moved": moved})
}

func (h *FileHandler) Copy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths       []string `json:"paths"`
		Destination string   `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if !h.checkAccess(r, req.Destination) {
		http.Error(w, "Access denied to destination", http.StatusForbidden)
		return
	}

	absDest, err := h.safePath(req.Destination)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	copied := 0
	for _, p := range req.Paths {
		if !h.checkAccess(r, filepath.ToSlash(filepath.Dir(p))) {
			continue
		}
		absSrc, err := h.safePath(p)
		if err != nil {
			continue
		}
		newPath := filepath.Join(absDest, filepath.Base(absSrc))
		if err := copyPath(absSrc, newPath); err == nil {
			copied++
		}
	}

	if h.audit != nil {
		h.audit.Log("COPY", middleware.GetUsername(r), getClientIP(r), fmt.Sprintf("copied %d item(s) to %s", copied, req.Destination))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"copied": copied})
}

func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()

	df, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer df.Close()

	_, err = io.Copy(df, sf)
	return err
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *FileHandler) Recent(w http.ResponseWriter, r *http.Request) {
	username := middleware.GetUsername(r)
	role := middleware.GetRole(r)
	homeFolder := middleware.GetHomeFolder(r)
	if homeFolder == "" {
		homeFolder = "/"
	}

	searchRoot := filepath.Join(h.root, homeFolder)
	if role == "admin" {
		searchRoot = h.root
	}

	type modFile struct {
		info FileInfo
		mod  int64
	}
	var recentFiles []modFile

	_ = filepath.Walk(searchRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		// Skip symlinks.
		if info.Mode()&os.ModeSymlink != 0 {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(h.root, path)
		entryPath := "/" + filepath.ToSlash(relPath)

		if h.permStore != nil && !h.permStore.CanAccess(filepath.Dir(entryPath), username, role) {
			return nil
		}

		recentFiles = append(recentFiles, modFile{
			info: FileInfo{
				Name:      info.Name(),
				Path:      entryPath,
				IsDir:     false,
				Size:      info.Size(),
				CreatedAt: getCreationTime(info),
				ModTime:   info.ModTime().UnixMilli(),
			},
			mod: info.ModTime().UnixMilli(),
		})
		return nil
	})

	sort.Slice(recentFiles, func(i, j int) bool {
		return recentFiles[i].mod > recentFiles[j].mod
	})

	limit := 30
	if len(recentFiles) > limit {
		recentFiles = recentFiles[:limit]
	}

	results := make([]FileInfo, len(recentFiles))
	for i, f := range recentFiles {
		results[i] = f.info
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (h *FileHandler) Extract(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if !h.checkAccess(r, filepath.ToSlash(filepath.Dir(req.Path))) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	absPath, err := h.safePath(req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(filepath.Ext(absPath))
	if ext != ".zip" {
		http.Error(w, "Only .zip files can be extracted", http.StatusBadRequest)
		return
	}

	destDir := filepath.Dir(absPath)
	reader, err := zip.OpenReader(absPath)
	if err != nil {
		http.Error(w, "Failed to open zip file", http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	cleanDest := filepath.Clean(destDir) + string(filepath.Separator)
	extracted := 0
	for _, f := range reader.File {
		fpath := filepath.Join(destDir, f.Name)
		cleaned := filepath.Clean(fpath)
		// Prevent zip slip — require the cleaned path to live under destDir
		// (with trailing separator to avoid /data being a prefix of /data-evil).
		if cleaned != filepath.Clean(destDir) && !strings.HasPrefix(cleaned+string(filepath.Separator), cleanDest) {
			slog.Warn("zip slip attempt blocked", "entry", f.Name, "dest", destDir)
			continue
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(fpath), 0755)
		outFile, err := os.Create(fpath)
		if err != nil {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			continue
		}
		io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		extracted++
	}

	if h.audit != nil {
		h.audit.Log("EXTRACT", middleware.GetUsername(r), getClientIP(r), fmt.Sprintf("extracted %s (%d files)", req.Path, extracted))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"extracted": extracted})
}

func (h *FileHandler) Compress(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths []string `json:"paths"`
		Name  string   `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if len(req.Paths) == 0 {
		http.Error(w, "No files specified", http.StatusBadRequest)
		return
	}

	// Create zip in same directory as first file
	firstDir := filepath.ToSlash(filepath.Dir(req.Paths[0]))
	if !h.checkAccess(r, firstDir) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	zipName := req.Name
	if zipName == "" {
		zipName = "archive.zip"
	}
	if !strings.HasSuffix(zipName, ".zip") {
		zipName += ".zip"
	}

	absDir, err := h.safePath(firstDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	zipPath := filepath.Join(absDir, zipName)
	zipFile, err := os.Create(zipPath)
	if err != nil {
		http.Error(w, "Failed to create zip", http.StatusInternalServerError)
		return
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	for _, p := range req.Paths {
		absSrc, err := h.safePath(p)
		if err != nil {
			continue
		}
		info, err := os.Stat(absSrc)
		if err != nil {
			continue
		}
		if info.IsDir() {
			walkFilesNoSymlinks(absSrc, func(fpath string, finfo os.FileInfo, _ string) {
				relPath, _ := filepath.Rel(absDir, fpath)
				header, _ := zip.FileInfoHeader(finfo)
				header.Name = filepath.ToSlash(relPath)
				header.Method = zip.Deflate
				writer, _ := zw.CreateHeader(header)
				f, err := os.Open(fpath)
				if err != nil {
					return
				}
				defer f.Close()
				if _, err := io.Copy(writer, f); err != nil {
					slog.Debug("compress copy failed", "err", err)
				}
			})
		} else {
			header, _ := zip.FileInfoHeader(info)
			header.Name = info.Name()
			header.Method = zip.Deflate
			writer, _ := zw.CreateHeader(header)
			f, err := os.Open(absSrc)
			if err != nil {
				continue
			}
			io.Copy(writer, f)
			f.Close()
		}
	}

	if h.audit != nil {
		h.audit.Log("COMPRESS", middleware.GetUsername(r), getClientIP(r), fmt.Sprintf("created %s/%s", firstDir, zipName))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"created": firstDir + "/" + zipName})
}

func (h *FileHandler) SetTags(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string   `json:"path"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if h.tags == nil {
		http.Error(w, "Tags not available", http.StatusInternalServerError)
		return
	}

	if err := h.tags.SetTags(req.Path, req.Tags); err != nil {
		http.Error(w, "Failed to set tags", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"path": req.Path, "tags": req.Tags})
}

func (h *FileHandler) GetTags(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if h.tags == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{})
		return
	}

	tags := h.tags.GetTags(path)
	if tags == nil {
		tags = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tags)
}

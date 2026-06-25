package handlers

import (
	"clouddrive/middleware"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// maxChunkBytes caps a single uploaded chunk (the whole-file cap is enforced at
// assembly time against the quota / MAX_UPLOAD_BYTES).
const maxChunkBytes = 64 << 20

var uploadIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

func validUploadID(id string) bool { return uploadIDRe.MatchString(id) }

// sessionDir returns the assembly directory for an upload, after verifying the
// caller owns it (first writer claims it via an .owner file).
func (h *FileHandler) claimUploadDir(uploadID, username string, create bool) (string, error) {
	dir := filepath.Join(h.uploadDir, uploadID)
	ownerFile := filepath.Join(dir, ".owner")
	if create {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", err
		}
		if _, err := os.Stat(ownerFile); os.IsNotExist(err) {
			_ = os.WriteFile(ownerFile, []byte(username), 0600)
		}
	}
	owner, err := os.ReadFile(ownerFile)
	if err != nil {
		return "", fmt.Errorf("unknown upload")
	}
	if string(owner) != username {
		return "", fmt.Errorf("not your upload")
	}
	return dir, nil
}

// UploadChunk stores one chunk of a resumable upload.
func (h *FileHandler) UploadChunk(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	uploadID := q.Get("uploadId")
	targetDir := q.Get("path")
	if !validUploadID(uploadID) {
		http.Error(w, "invalid uploadId", http.StatusBadRequest)
		return
	}
	idx, err := strconv.Atoi(q.Get("index"))
	if err != nil || idx < 0 {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}
	if targetDir == "" {
		targetDir = "/"
	}
	if !h.checkAccess(r, targetDir) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	dir, err := h.claimUploadDir(uploadID, middleware.GetUsername(r), true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxChunkBytes)
	// Write to a temp file then rename, so a partially-received chunk isn't
	// mistaken for complete on resume.
	tmp := filepath.Join(dir, fmt.Sprintf("%d.part.tmp", idx))
	f, err := os.Create(tmp)
	if err != nil {
		http.Error(w, "cannot store chunk", http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(f, r.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		http.Error(w, "chunk too large or interrupted", http.StatusRequestEntityTooLarge)
		return
	}
	f.Close()
	if err := os.Rename(tmp, filepath.Join(dir, fmt.Sprintf("%d.part", idx))); err != nil {
		http.Error(w, "cannot store chunk", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{"received": idx})
}

// UploadStatus reports which chunk indices have been received (for resume).
func (h *FileHandler) UploadStatus(w http.ResponseWriter, r *http.Request) {
	uploadID := r.URL.Query().Get("uploadId")
	if !validUploadID(uploadID) {
		http.Error(w, "invalid uploadId", http.StatusBadRequest)
		return
	}
	dir, err := h.claimUploadDir(uploadID, middleware.GetUsername(r), false)
	if err != nil {
		// Unknown upload → nothing received yet.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"received": []int{}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"received": receivedIndices(dir)})
}

func receivedIndices(dir string) []int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []int{}
	}
	out := []int{}
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".part") {
			if n, err := strconv.Atoi(strings.TrimSuffix(name, ".part")); err == nil {
				out = append(out, n)
			}
		}
	}
	return out
}

// UploadComplete assembles the received chunks into the final file.
func (h *FileHandler) UploadComplete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UploadID string `json:"uploadId"`
		Name     string `json:"name"`
		Path     string `json:"path"`
		Total    int    `json:"total"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if !validUploadID(req.UploadID) || req.Total <= 0 {
		http.Error(w, "invalid upload", http.StatusBadRequest)
		return
	}
	if req.Path == "" {
		req.Path = "/"
	}
	if !h.checkAccess(r, req.Path) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	name := filepath.Base(req.Name)
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, ".") {
		http.Error(w, "invalid file name", http.StatusBadRequest)
		return
	}
	absDir, err := h.safePath(req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dir, err := h.claimUploadDir(req.UploadID, middleware.GetUsername(r), false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// All chunks must be present, and we need the assembled size for quota.
	var assembled int64
	for i := 0; i < req.Total; i++ {
		info, err := os.Stat(filepath.Join(dir, fmt.Sprintf("%d.part", i)))
		if err != nil {
			http.Error(w, fmt.Sprintf("missing chunk %d", i), http.StatusBadRequest)
			return
		}
		assembled += info.Size()
	}

	// Quota check on the assembled size.
	if h.quotaOf != nil {
		if quota := h.quotaOf(middleware.GetUsername(r)); quota > 0 {
			home := middleware.GetHomeFolder(r)
			if home == "" {
				home = "/"
			}
			if homeAbs, err := h.safePath(home); err == nil {
				if dirSize(homeAbs)+assembled > quota {
					http.Error(w, "Storage quota exceeded", http.StatusInsufficientStorage)
					return
				}
			}
		}
	}

	dstPath := filepath.Join(absDir, name)
	// Snapshot an existing file before overwriting (versioning).
	if h.versions != nil {
		if _, statErr := os.Stat(dstPath); statErr == nil {
			_ = h.versions.SaveVersion(dstPath, filepath.ToSlash(filepath.Join(req.Path, name)))
		}
	}

	// Assemble into a temp file then rename for atomicity.
	tmpOut := dstPath + ".assembling"
	out, err := os.Create(tmpOut)
	if err != nil {
		http.Error(w, "cannot create file", http.StatusInternalServerError)
		return
	}
	for i := 0; i < req.Total; i++ {
		part, err := os.Open(filepath.Join(dir, fmt.Sprintf("%d.part", i)))
		if err != nil {
			out.Close()
			os.Remove(tmpOut)
			http.Error(w, "assembly failed", http.StatusInternalServerError)
			return
		}
		if _, err := io.Copy(out, part); err != nil {
			part.Close()
			out.Close()
			os.Remove(tmpOut)
			http.Error(w, "assembly failed", http.StatusInternalServerError)
			return
		}
		part.Close()
	}
	out.Close()
	if err := os.Rename(tmpOut, dstPath); err != nil {
		os.Remove(tmpOut)
		http.Error(w, "assembly failed", http.StatusInternalServerError)
		return
	}
	os.RemoveAll(dir) // clean up the chunk staging area

	if h.audit != nil {
		h.audit.Log("UPLOAD", middleware.GetUsername(r), getClientIP(r), fmt.Sprintf("uploaded %s to %s (chunked, %d bytes)", name, req.Path, assembled))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"uploaded": name, "size": assembled})
}

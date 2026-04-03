package handlers

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ShareEntry struct {
	Token     string `json:"token"`
	FilePath  string `json:"filePath"`
	FileName  string `json:"fileName"`
	IsDir     bool   `json:"isDir"`
	CreatedAt int64  `json:"createdAt"`
	ExpiresAt int64  `json:"expiresAt"`
}

type ShareHandler struct {
	root   string
	shares map[string]*ShareEntry
	mu     sync.RWMutex
}

func NewShareHandler(root string) *ShareHandler {
	return &ShareHandler{
		root:   root,
		shares: make(map[string]*ShareEntry),
	}
}

func (h *ShareHandler) safePath(reqPath string) (string, error) {
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

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Create generates a share link (requires auth — called from frontend)
func (h *ShareHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path      string `json:"path"`
		ExpiresIn int    `json:"expiresIn"` // hours, 0 = 7 days default
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	absPath, err := h.safePath(req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	token, err := generateToken()
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	expiresIn := time.Duration(req.ExpiresIn) * time.Hour
	if expiresIn <= 0 {
		expiresIn = 7 * 24 * time.Hour // 7 days default
	}

	entry := &ShareEntry{
		Token:     token,
		FilePath:  req.Path,
		FileName:  info.Name(),
		IsDir:     info.IsDir(),
		CreatedAt: time.Now().UnixMilli(),
		ExpiresAt: time.Now().Add(expiresIn).UnixMilli(),
	}

	h.mu.Lock()
	h.shares[token] = entry
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": token,
		"url":   fmt.Sprintf("/share/%s", token),
	})
}

// List returns all active shares (requires auth)
func (h *ShareHandler) List(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	now := time.Now().UnixMilli()
	active := make([]*ShareEntry, 0)
	for _, entry := range h.shares {
		if entry.ExpiresAt > now {
			active = append(active, entry)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(active)
}

// Revoke deletes a share (requires auth)
func (h *ShareHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	delete(h.shares, req.Token)
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"revoked": req.Token})
}

// Download serves the shared file (NO auth — public endpoint)
func (h *ShareHandler) Download(w http.ResponseWriter, r *http.Request) {
	// Extract token from URL: /share/{token}
	token := strings.TrimPrefix(r.URL.Path, "/share/")
	token = strings.Split(token, "/")[0]
	if token == "" {
		http.Error(w, "Invalid share link", http.StatusBadRequest)
		return
	}

	h.mu.RLock()
	entry, exists := h.shares[token]
	h.mu.RUnlock()

	if !exists {
		http.Error(w, "Share link not found or expired", http.StatusNotFound)
		return
	}

	if time.Now().UnixMilli() > entry.ExpiresAt {
		h.mu.Lock()
		delete(h.shares, token)
		h.mu.Unlock()
		http.Error(w, "Share link has expired", http.StatusGone)
		return
	}

	absPath, err := h.safePath(entry.FilePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	if info.IsDir() {
		h.serveDirectoryAsZip(w, absPath, entry.FileName)
	} else {
		h.serveFile(w, absPath, info)
	}
}

func (h *ShareHandler) serveFile(w http.ResponseWriter, absPath string, info os.FileInfo) {
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

func (h *ShareHandler) serveDirectoryAsZip(w http.ResponseWriter, absPath string, dirName string) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, dirName))

	zw := zip.NewWriter(w)
	defer zw.Close()

	filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(absPath, path)
		if err != nil {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return nil
		}
		header.Name = filepath.ToSlash(filepath.Join(dirName, relPath))
		header.Method = zip.Deflate

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		io.Copy(writer, f)
		return nil
	})
}

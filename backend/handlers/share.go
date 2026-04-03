package handlers

import (
	"archive/zip"
	"clouddrive/middleware"
	"clouddrive/services"
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
	Password  string `json:"password,omitempty"`
	CreatedAt int64  `json:"createdAt"`
	ExpiresAt int64  `json:"expiresAt"`
}

type ShareHandler struct {
	root      string
	storePath string
	shares    map[string]*ShareEntry
	mu        sync.RWMutex
	audit     *services.AuditLogger
}

func NewShareHandler(root string, audit *services.AuditLogger) *ShareHandler {
	h := &ShareHandler{
		root:      root,
		audit:     audit,
		storePath: filepath.Join(root, ".shares.json"),
		shares:    make(map[string]*ShareEntry),
	}
	h.load()
	h.cleanExpired()
	return h
}

func (h *ShareHandler) load() {
	data, err := os.ReadFile(h.storePath)
	if err != nil {
		return // file doesn't exist yet
	}
	json.Unmarshal(data, &h.shares)
}

func (h *ShareHandler) save() {
	data, err := json.MarshalIndent(h.shares, "", "  ")
	if err != nil {
		return
	}
	tmpPath := h.storePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return
	}
	os.Rename(tmpPath, h.storePath)
}

func (h *ShareHandler) cleanExpired() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now().UnixMilli()
	changed := false
	for token, entry := range h.shares {
		if entry.ExpiresAt < now {
			delete(h.shares, token)
			changed = true
		}
	}
	if changed {
		h.save()
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

func generatePassword() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Create generates a share link (requires auth — called from frontend)
func (h *ShareHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path      string `json:"path"`
		Safe      bool   `json:"safe"`
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

	var password string
	if req.Safe {
		password, err = generatePassword()
		if err != nil {
			http.Error(w, "Failed to generate password", http.StatusInternalServerError)
			return
		}
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
		Password:  password,
		CreatedAt: time.Now().UnixMilli(),
		ExpiresAt: time.Now().Add(expiresIn).UnixMilli(),
	}

	h.mu.Lock()
	h.shares[token] = entry
	h.save()
	h.mu.Unlock()

	if h.audit != nil {
		shareType := "share"
		if password != "" {
			shareType = "safe share"
		}
		h.audit.Log("SHARE", middleware.GetUsername(r), getClientIP(r), fmt.Sprintf("created %s for %s", shareType, req.Path))
	}

	resp := map[string]string{
		"token": token,
		"url":   fmt.Sprintf("/share/%s", token),
	}
	if password != "" {
		resp["password"] = password
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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
	h.save()
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"revoked": req.Token})
}

// Download serves the shared file (NO auth — public endpoint)
// For password-protected shares, accepts ?p=<password> or shows a password form
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
		h.save()
		h.mu.Unlock()
		http.Error(w, "Share link has expired", http.StatusGone)
		return
	}

	// Check password if required
	if entry.Password != "" {
		providedPassword := r.URL.Query().Get("p")

		// POST form submission
		if r.Method == "POST" {
			r.ParseForm()
			providedPassword = r.FormValue("password")
		}

		if providedPassword == "" {
			h.servePasswordPage(w, token, entry.FileName, false)
			return
		}
		if providedPassword != entry.Password {
			h.servePasswordPage(w, token, entry.FileName, true)
			return
		}
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

func (h *ShareHandler) servePasswordPage(w http.ResponseWriter, token string, fileName string, wrongPassword bool) {
	errorHTML := ""
	if wrongPassword {
		errorHTML = `<div style="color:#ef4444;background:#fef2f2;padding:8px 12px;border-radius:8px;font-size:14px;margin-bottom:16px">Incorrect password</div>`
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>CloudDrive — Protected Share</title>
	<style>
		* { margin: 0; padding: 0; box-sizing: border-box; }
		body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f3f4f6; display: flex; align-items: center; justify-content: center; min-height: 100vh; }
		.card { background: white; padding: 32px; border-radius: 16px; box-shadow: 0 4px 24px rgba(0,0,0,0.08); width: 100%%; max-width: 400px; margin: 16px; }
		h1 { font-size: 20px; color: #1f2937; margin-bottom: 4px; }
		.subtitle { color: #6b7280; font-size: 14px; margin-bottom: 24px; }
		.filename { color: #374151; font-weight: 500; }
		label { display: block; font-size: 14px; font-weight: 500; color: #374151; margin-bottom: 6px; }
		input { width: 100%%; padding: 10px 14px; border: 1px solid #d1d5db; border-radius: 8px; font-size: 14px; outline: none; transition: border-color 0.2s; }
		input:focus { border-color: #3b82f6; box-shadow: 0 0 0 3px rgba(59,130,246,0.1); }
		button { width: 100%%; padding: 10px; background: #2563eb; color: white; border: none; border-radius: 8px; font-size: 14px; font-weight: 500; cursor: pointer; margin-top: 16px; transition: background 0.2s; }
		button:hover { background: #1d4ed8; }
	</style>
</head>
<body>
	<div class="card">
		<h1>Protected File</h1>
		<p class="subtitle">Enter the password to download <span class="filename">%s</span></p>
		%s
		<form method="POST" action="/share/%s">
			<label for="password">Password</label>
			<input type="password" name="password" id="password" placeholder="Enter password" autofocus required />
			<button type="submit">Download</button>
		</form>
	</div>
</body>
</html>`, fileName, errorHTML, token)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
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

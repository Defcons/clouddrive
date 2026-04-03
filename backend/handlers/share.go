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
	Token      string `json:"token"`
	FilePath   string `json:"filePath"`
	FileName   string `json:"fileName"`
	IsDir      bool   `json:"isDir"`
	Mode       string `json:"mode"` // "download" or "collaborate"
	Password   string `json:"password,omitempty"`
	CreatedBy  string `json:"createdBy"`
	CreatedAt  int64  `json:"createdAt"`
	ExpiresAt  int64  `json:"expiresAt"`
	Downloads  int    `json:"downloads"`
	LastAccess int64  `json:"lastAccess,omitempty"`
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
		Mode      string `json:"mode"`      // "download" or "collaborate"
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

	mode := req.Mode
	if mode == "" {
		mode = "download"
	}

	entry := &ShareEntry{
		Token:     token,
		FilePath:  req.Path,
		FileName:  info.Name(),
		IsDir:     info.IsDir(),
		Mode:      mode,
		Password:  password,
		CreatedBy: middleware.GetUsername(r),
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

// Download serves the shared file/folder (NO auth — public endpoint)
func (h *ShareHandler) Download(w http.ResponseWriter, r *http.Request) {
	// Extract token and subpath from URL: /share/{token}/subpath...
	remainder := strings.TrimPrefix(r.URL.Path, "/share/")
	parts := strings.SplitN(remainder, "/", 2)
	token := parts[0]
	subPath := ""
	if len(parts) > 1 {
		subPath = parts[1]
	}

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

	// Track access
	h.mu.Lock()
	entry.LastAccess = time.Now().UnixMilli()
	entry.Downloads++
	h.save()
	h.mu.Unlock()

	absPath, err := h.safePath(entry.FilePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// If subPath, resolve within the shared directory
	targetPath := absPath
	if subPath != "" {
		targetPath = filepath.Join(absPath, filepath.Clean(subPath))
		// Prevent traversal outside shared path
		if !strings.HasPrefix(targetPath, absPath) {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// ?download=1 forces download (for files or zip of folder)
	if r.URL.Query().Get("download") == "1" {
		if info.IsDir() {
			h.serveDirectoryAsZip(w, targetPath, info.Name())
		} else {
			h.serveFile(w, targetPath, info)
		}
		return
	}

	// Single file — serve directly
	if !info.IsDir() {
		h.serveFile(w, targetPath, info)
		return
	}

	// Directory — serve browseable HTML
	h.serveBrowsePage(w, token, entry, targetPath, subPath)
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

// Upload handles file uploads to collaborative shared folders (NO auth — public)
func (h *ShareHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract token from URL: /share/{token}/upload
	remainder := strings.TrimPrefix(r.URL.Path, "/share/")
	parts := strings.SplitN(remainder, "/", 2)
	token := parts[0]

	h.mu.RLock()
	entry, exists := h.shares[token]
	h.mu.RUnlock()

	if !exists || time.Now().UnixMilli() > entry.ExpiresAt {
		http.Error(w, "Share link not found or expired", http.StatusNotFound)
		return
	}

	if entry.Mode != "collaborate" {
		http.Error(w, "This share does not allow uploads", http.StatusForbidden)
		return
	}

	// Check password
	if entry.Password != "" {
		p := r.URL.Query().Get("p")
		if p != entry.Password {
			http.Error(w, "Invalid password", http.StatusUnauthorized)
			return
		}
	}

	absPath, err := h.safePath(entry.FilePath)
	if err != nil {
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}

	// Handle optional subpath
	subPath := r.URL.Query().Get("path")
	targetDir := absPath
	if subPath != "" {
		targetDir = filepath.Join(absPath, filepath.Clean(subPath))
		if !strings.HasPrefix(targetDir, absPath) {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
	}

	r.ParseMultipartForm(500 << 20)
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "No files provided", http.StatusBadRequest)
		return
	}

	uploaded := 0
	for _, fh := range files {
		src, err := fh.Open()
		if err != nil {
			continue
		}
		dstPath := filepath.Join(targetDir, filepath.Base(fh.Filename))
		dst, err := os.Create(dstPath)
		if err != nil {
			src.Close()
			continue
		}
		io.Copy(dst, src)
		src.Close()
		dst.Close()
		uploaded++
	}

	if h.audit != nil {
		h.audit.Log("SHARE_UPLOAD", "anonymous", r.RemoteAddr, fmt.Sprintf("uploaded %d file(s) via share %s", uploaded, token))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"uploaded": uploaded})
}

func (h *ShareHandler) serveBrowsePage(w http.ResponseWriter, token string, entry *ShareEntry, absPath string, subPath string) {
	entries, err := os.ReadDir(absPath)
	if err != nil {
		http.Error(w, "Cannot read directory", http.StatusInternalServerError)
		return
	}

	type fileEntry struct {
		Name  string
		IsDir bool
		Size  string
		Link  string
	}

	var items []fileEntry
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		link := "/share/" + token + "/"
		if subPath != "" {
			link += subPath + "/"
		}
		link += e.Name()

		size := ""
		if !e.IsDir() {
			bytes := info.Size()
			if bytes < 1024 {
				size = fmt.Sprintf("%d B", bytes)
			} else if bytes < 1024*1024 {
				size = fmt.Sprintf("%.1f KB", float64(bytes)/1024)
			} else if bytes < 1024*1024*1024 {
				size = fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
			} else {
				size = fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
			}
		}

		items = append(items, fileEntry{Name: e.Name(), IsDir: e.IsDir(), Size: size, Link: link})
	}

	// Build breadcrumb
	breadcrumb := entry.FileName
	if subPath != "" {
		breadcrumb += " / " + strings.ReplaceAll(subPath, "/", " / ")
	}

	parentLink := ""
	if subPath != "" {
		parentParts := strings.Split(subPath, "/")
		if len(parentParts) > 1 {
			parentLink = "/share/" + token + "/" + strings.Join(parentParts[:len(parentParts)-1], "/")
		} else {
			parentLink = "/share/" + token
		}
	}

	// Build file list HTML
	var fileRows string
	if parentLink != "" {
		fileRows += fmt.Sprintf(`<a href="%s" class="item"><span class="icon">📁</span><span class="name">..</span><span class="size"></span></a>`, parentLink)
	}
	for _, item := range items {
		icon := "📄"
		if item.IsDir {
			icon = "📁"
		}
		fileRows += fmt.Sprintf(`<a href="%s" class="item"><span class="icon">%s</span><span class="name">%s</span><span class="size">%s</span></a>`,
			item.Link, icon, item.Name, item.Size)
	}

	downloadAllLink := "/share/" + token
	if subPath != "" {
		downloadAllLink += "/" + subPath
	}
	downloadAllLink += "?download=1"

	uploadSection := ""
	if entry.Mode == "collaborate" {
		uploadAction := "/share/" + token + "/upload"
		pwParam := ""
		if entry.Password != "" {
			pwParam = "?p=" + entry.Password
		}
		if subPath != "" {
			if pwParam != "" {
				pwParam += "&path=" + subPath
			} else {
				pwParam = "?path=" + subPath
			}
		}
		uploadSection = fmt.Sprintf(`
<div class="upload-zone" id="upload-zone">
  <form method="POST" action="%s%s" enctype="multipart/form-data" class="upload-form">
    <input type="file" name="files" multiple id="file-input" style="display:none" onchange="this.form.submit()">
    <button type="button" onclick="document.getElementById('file-input').click()" class="dl-btn" style="background:#16a34a">Upload Files</button>
    <span class="upload-hint">or drag files here</span>
  </form>
</div>
<script>
const zone=document.getElementById('upload-zone');
zone.addEventListener('dragover',e=>{e.preventDefault();zone.style.background='#eff6ff'});
zone.addEventListener('dragleave',()=>{zone.style.background=''});
zone.addEventListener('drop',e=>{
  e.preventDefault();zone.style.background='';
  const fd=new FormData();
  for(const f of e.dataTransfer.files)fd.append('files',f);
  fetch('%s%s',{method:'POST',body:fd}).then(()=>location.reload());
});
</script>`, uploadAction, pwParam, uploadAction, pwParam)
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>CloudDrive — %s</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#f8fafc;color:#1e293b;min-height:100vh}
.header{background:#fff;border-bottom:1px solid #e2e8f0;padding:16px 24px;display:flex;align-items:center;justify-content:space-between}
.header h1{font-size:16px;font-weight:600;color:#334155}
.header .meta{font-size:12px;color:#94a3b8}
.dl-btn{padding:8px 16px;background:#2563eb;color:#fff;border:none;border-radius:8px;font-size:13px;cursor:pointer;text-decoration:none}
.dl-btn:hover{background:#1d4ed8}
.breadcrumb{padding:12px 24px;font-size:13px;color:#64748b;border-bottom:1px solid #f1f5f9}
.list{padding:8px 16px}
.item{display:flex;align-items:center;padding:10px 12px;text-decoration:none;color:#334155;border-radius:8px;transition:background 0.1s}
.item:hover{background:#f1f5f9}
.icon{width:24px;font-size:18px;flex-shrink:0}
.name{flex:1;font-size:14px}
.size{font-size:12px;color:#94a3b8;min-width:80px;text-align:right}
.upload-zone{padding:16px 24px;border-top:1px solid #e2e8f0;display:flex;align-items:center;gap:12px}
.upload-form{display:flex;align-items:center;gap:12px}
.upload-hint{font-size:13px;color:#94a3b8}
.footer{padding:16px 24px;text-align:center;font-size:11px;color:#cbd5e1;margin-top:32px}
</style>
</head>
<body>
<div class="header">
  <div>
    <h1>%s</h1>
    <div class="meta">Shared via CloudDrive</div>
  </div>
  <a href="%s" class="dl-btn">Download All (ZIP)</a>
</div>
<div class="breadcrumb">%s</div>
<div class="list">%s</div>
%s
<div class="footer">%d file(s) &middot; Powered by CloudDrive</div>
</body>
</html>`, breadcrumb, entry.FileName, downloadAllLink, breadcrumb, fileRows, uploadSection, len(items))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

package handlers

import (
	"archive/zip"
	"clouddrive/middleware"
	"clouddrive/services"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
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
	Mode      string `json:"mode"` // "download" or "collaborate"
	Password  string `json:"password,omitempty"`
	CreatedBy string `json:"createdBy"`
	CreatedAt int64  `json:"createdAt"`
	ExpiresAt int64  `json:"expiresAt"`
	Downloads int    `json:"downloads"`
}

type ShareRateLimiter interface {
	Check(r *http.Request) bool
}

type ShareHandler struct {
	root      string
	storePath string
	shares    map[string]*ShareEntry
	mu        sync.RWMutex
	audit     *services.AuditLogger
	pwLimiter ShareRateLimiter
}

func NewShareHandler(root string, audit *services.AuditLogger, pwLimiter ShareRateLimiter) *ShareHandler {
	h := &ShareHandler{
		root:      root,
		audit:     audit,
		pwLimiter: pwLimiter,
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
		return
	}
	if err := json.Unmarshal(data, &h.shares); err != nil {
		slog.Warn("failed to parse shares store", "err", err)
	}
}

func (h *ShareHandler) save() {
	data, err := json.MarshalIndent(h.shares, "", "  ")
	if err != nil {
		slog.Warn("failed to marshal shares", "err", err)
		return
	}
	tmpPath := h.storePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		slog.Warn("failed to write shares tmp", "err", err)
		return
	}
	if err := os.Rename(tmpPath, h.storePath); err != nil {
		slog.Warn("failed to rename shares tmp", "err", err)
	}
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

// safePath returns the absolute filesystem path for reqPath, rejecting any
// traversal outside the storage root AND any symlinks that cross the boundary.
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
	// Resolve symlinks to catch cases where a link points outside the root.
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		abs = resolved
	}
	if !strings.HasPrefix(abs+string(filepath.Separator), rootAbs+string(filepath.Separator)) && abs != rootAbs {
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

// generatePassword returns a 128-bit hex string (32 chars).
// Previously only 32 bits — brute-forceable in seconds.
func generatePassword() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (h *ShareHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path      string `json:"path"`
		Safe      bool   `json:"safe"`
		Mode      string `json:"mode"`
		ExpiresIn int    `json:"expiresIn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	absPath, err := h.safePath(req.Path)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
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
		expiresIn = 7 * 24 * time.Hour
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
	_ = json.NewEncoder(w).Encode(resp)
}

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
	_ = json.NewEncoder(w).Encode(active)
}

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
	_ = json.NewEncoder(w).Encode(map[string]string{"revoked": req.Token})
}

// Download serves the shared file/folder. Password checks use rate limiting
// + constant-time compare, and passwords are NEVER included in URLs.
func (h *ShareHandler) Download(w http.ResponseWriter, r *http.Request) {
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

	// Password gate (if set).
	if entry.Password != "" {
		// Rate limit guesses.
		if h.pwLimiter != nil && !h.pwLimiter.Check(r) {
			http.Error(w, "Too many attempts. Try again later.", http.StatusTooManyRequests)
			return
		}
		// Accept password ONLY via form POST body or session cookie.
		// Do NOT accept ?p=... query params (leak in logs/history).
		providedPassword := ""
		if r.Method == "POST" {
			_ = r.ParseForm()
			providedPassword = r.FormValue("password")
		} else if c, err := r.Cookie(shareAuthCookieName(token)); err == nil {
			providedPassword = c.Value
		}
		if providedPassword == "" {
			h.servePasswordPage(w, token, entry.FileName, false)
			return
		}
		if subtle.ConstantTimeCompare([]byte(providedPassword), []byte(entry.Password)) != 1 {
			h.servePasswordPage(w, token, entry.FileName, true)
			return
		}
		// Good password via POST → set session cookie so follow-up nav doesn't re-prompt.
		if r.Method == "POST" {
			http.SetCookie(w, &http.Cookie{
				Name:     shareAuthCookieName(token),
				Value:    entry.Password,
				Path:     "/share/" + token,
				HttpOnly: true,
				Secure:   os.Getenv("COOKIE_INSECURE") != "1",
				SameSite: http.SameSiteLaxMode,
				MaxAge:   3600,
			})
			// Redirect to GET so a refresh doesn't resubmit the password form.
			http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
			return
		}
	}

	h.mu.Lock()
	entry.Downloads++
	h.save()
	h.mu.Unlock()

	absPath, err := h.safePath(entry.FilePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	targetPath := absPath
	if subPath != "" {
		candidate := filepath.Join(absPath, filepath.Clean(subPath))
		// Resolve symlinks to block escape via symlink.
		if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
			candidate = resolved
		}
		if !strings.HasPrefix(candidate+string(filepath.Separator), absPath+string(filepath.Separator)) && candidate != absPath {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
		targetPath = candidate
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	if r.URL.Query().Get("download") == "1" {
		if info.IsDir() {
			h.serveDirectoryAsZip(w, targetPath, info.Name())
		} else {
			h.serveFile(w, targetPath, info)
		}
		return
	}

	if !info.IsDir() {
		h.serveFile(w, targetPath, info)
		return
	}

	h.serveBrowsePage(w, token, entry, targetPath, subPath)
}

func shareAuthCookieName(token string) string {
	return "share_auth_" + token[:8]
}

func (h *ShareHandler) servePasswordPage(w http.ResponseWriter, token string, fileName string, wrongPassword bool) {
	errorHTML := ""
	if wrongPassword {
		errorHTML = `<div style="color:#ef4444;background:#fef2f2;padding:8px 12px;border-radius:8px;font-size:14px;margin-bottom:16px">Incorrect password</div>`
	}

	safeName := html.EscapeString(fileName)
	safeToken := html.EscapeString(token)

	pageHTML := fmt.Sprintf(`<!DOCTYPE html>
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
		input { width: 100%%; padding: 12px 14px; border: 1px solid #d1d5db; border-radius: 8px; font-size: 14px; outline: none; transition: border-color 0.2s; min-height: 44px; }
		input:focus { border-color: #3b82f6; box-shadow: 0 0 0 3px rgba(59,130,246,0.1); }
		button { width: 100%%; padding: 12px; background: #2563eb; color: white; border: none; border-radius: 8px; font-size: 14px; font-weight: 500; cursor: pointer; margin-top: 16px; transition: background 0.2s; min-height: 44px; }
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
			<input type="password" name="password" id="password" placeholder="Enter password" autofocus required autocomplete="current-password" />
			<button type="submit">Unlock</button>
		</form>
	</div>
</body>
</html>`, safeName, errorHTML, safeToken)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(pageHTML))
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
	if _, err := io.Copy(w, f); err != nil {
		slog.Debug("serveFile copy failed (client disconnect?)", "err", err)
	}
}

func (h *ShareHandler) serveDirectoryAsZip(w http.ResponseWriter, absPath string, dirName string) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, dirName))

	zw := zip.NewWriter(w)
	defer zw.Close()

	walkFilesNoSymlinks(absPath, func(path string, info os.FileInfo, relPath string) {
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return
		}
		header.Name = filepath.ToSlash(filepath.Join(dirName, relPath))
		header.Method = zip.Deflate

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return
		}
		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()
		_, _ = io.Copy(writer, f)
	})
}

func (h *ShareHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

	if entry.Password != "" {
		// Accept password only via session cookie (set after form POST).
		// Never via ?p=... query params.
		if h.pwLimiter != nil && !h.pwLimiter.Check(r) {
			http.Error(w, "Too many attempts. Try again later.", http.StatusTooManyRequests)
			return
		}
		c, err := r.Cookie(shareAuthCookieName(token))
		if err != nil || subtle.ConstantTimeCompare([]byte(c.Value), []byte(entry.Password)) != 1 {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
	}

	absPath, err := h.safePath(entry.FilePath)
	if err != nil {
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}

	subPath := r.URL.Query().Get("path")
	targetDir := absPath
	if subPath != "" {
		candidate := filepath.Join(absPath, filepath.Clean(subPath))
		if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
			candidate = resolved
		}
		if !strings.HasPrefix(candidate+string(filepath.Separator), absPath+string(filepath.Separator)) && candidate != absPath {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
		targetDir = candidate
	}

	if err := r.ParseMultipartForm(500 << 20); err != nil {
		http.Error(w, "Failed to parse upload", http.StatusBadRequest)
		return
	}
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
		if _, err := io.Copy(dst, src); err != nil {
			slog.Warn("share upload copy failed", "err", err, "file", fh.Filename)
		}
		src.Close()
		dst.Close()
		uploaded++
	}

	if h.audit != nil {
		h.audit.Log("SHARE_UPLOAD", "anonymous", r.RemoteAddr, fmt.Sprintf("uploaded %d file(s) via share %s", uploaded, token))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{"uploaded": uploaded})
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
		// Skip symlinks in directory listings — they may point outside the share.
		if e.Type()&os.ModeSymlink != 0 {
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
			switch {
			case bytes < 1024:
				size = fmt.Sprintf("%d B", bytes)
			case bytes < 1024*1024:
				size = fmt.Sprintf("%.1f KB", float64(bytes)/1024)
			case bytes < 1024*1024*1024:
				size = fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
			default:
				size = fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
			}
		}

		items = append(items, fileEntry{Name: e.Name(), IsDir: e.IsDir(), Size: size, Link: link})
	}

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

	var fileRows string
	if parentLink != "" {
		fileRows += fmt.Sprintf(`<a href="%s" class="item"><span class="icon">📁</span><span class="name">..</span><span class="size"></span></a>`,
			html.EscapeString(parentLink))
	}
	for _, item := range items {
		icon := "📄"
		if item.IsDir {
			icon = "📁"
		}
		fileRows += fmt.Sprintf(`<a href="%s" class="item"><span class="icon">%s</span><span class="name">%s</span><span class="size">%s</span></a>`,
			html.EscapeString(item.Link), icon, html.EscapeString(item.Name), html.EscapeString(item.Size))
	}

	downloadAllLink := "/share/" + token
	if subPath != "" {
		downloadAllLink += "/" + subPath
	}
	downloadAllLink += "?download=1"

	// Upload section — password NOT exposed in URL; session cookie set earlier
	// carries auth for collaborate shares.
	uploadSection := ""
	if entry.Mode == "collaborate" {
		uploadAction := "/share/" + token + "/upload"
		pathParam := ""
		if subPath != "" {
			pathParam = "?path=" + subPath
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
  fetch(%q,{method:'POST',body:fd,credentials:'include'}).then(()=>location.reload());
});
</script>`, html.EscapeString(uploadAction), html.EscapeString(pathParam), uploadAction+pathParam)
	}

	pageHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>CloudDrive — %s</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#f8fafc;color:#1e293b;min-height:100vh}
.header{background:#fff;border-bottom:1px solid #e2e8f0;padding:16px 24px;display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}
.header h1{font-size:16px;font-weight:600;color:#334155;word-break:break-word}
.header .meta{font-size:12px;color:#94a3b8}
.dl-btn{padding:10px 16px;background:#2563eb;color:#fff;border:none;border-radius:8px;font-size:13px;cursor:pointer;text-decoration:none;min-height:44px;display:inline-flex;align-items:center;justify-content:center}
.dl-btn:hover{background:#1d4ed8}
.breadcrumb{padding:12px 24px;font-size:13px;color:#64748b;border-bottom:1px solid #f1f5f9;word-break:break-word}
.list{padding:8px 16px}
.item{display:flex;align-items:center;padding:12px;text-decoration:none;color:#334155;border-radius:8px;transition:background 0.1s;min-height:44px}
.item:hover{background:#f1f5f9}
.icon{width:24px;font-size:18px;flex-shrink:0}
.name{flex:1;font-size:14px;word-break:break-all}
.size{font-size:12px;color:#94a3b8;min-width:80px;text-align:right}
.upload-zone{padding:16px 24px;border-top:1px solid #e2e8f0;display:flex;align-items:center;gap:12px;flex-wrap:wrap}
.upload-form{display:flex;align-items:center;gap:12px;flex-wrap:wrap}
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
</html>`, html.EscapeString(breadcrumb), html.EscapeString(entry.FileName), html.EscapeString(downloadAllLink), html.EscapeString(breadcrumb), fileRows, uploadSection, len(items))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(pageHTML))
}

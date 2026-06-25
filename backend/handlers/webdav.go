package handlers

import (
	"clouddrive/services"
	"net/http"
	"path/filepath"

	"golang.org/x/net/webdav"
)

// WebDAVHandler exposes the storage over WebDAV so it can be mounted as a
// native network drive. It authenticates with HTTP Basic Auth against the user
// store and scopes each user to their home folder (admins get the whole root).
//
// NOTE: WebDAV uses Basic Auth (password only) — it does NOT enforce MFA. It is
// opt-in via WEBDAV_ENABLED=1 for that reason; serve it only over HTTPS.
type WebDAVHandler struct {
	root      string
	userStore *services.UserStore
	lockSys   webdav.LockSystem
}

const webDAVPrefix = "/webdav"

func NewWebDAVHandler(root string, userStore *services.UserStore) *WebDAVHandler {
	return &WebDAVHandler{
		root:      root,
		userStore: userStore,
		lockSys:   webdav.NewMemLS(),
	}
}

func (h *WebDAVHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok {
		h.requireAuth(w)
		return
	}
	user, err := h.userStore.Authenticate(username, password)
	if err != nil {
		h.requireAuth(w)
		return
	}

	// Scope the filesystem to the user's home folder (admins see everything).
	scope := h.root
	if user.Role != "admin" && user.HomeFolder != "" && user.HomeFolder != "/" {
		scope = filepath.Join(h.root, user.HomeFolder)
	}

	dav := &webdav.Handler{
		Prefix:     webDAVPrefix,
		FileSystem: webdav.Dir(scope),
		LockSystem: h.lockSys,
	}
	dav.ServeHTTP(w, r)
}

func (h *WebDAVHandler) requireAuth(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="CloudDrive WebDAV"`)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

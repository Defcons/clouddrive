package handlers

import (
	"clouddrive/middleware"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type DiskHandler struct {
	root    string
	quotaOf func(username string) int64
}

// SetQuotaLookup wires a per-user quota source so the caller's quota is
// reported alongside usage.
func (h *DiskHandler) SetQuotaLookup(fn func(username string) int64) {
	h.quotaOf = fn
}

type UserUsage struct {
	Username string `json:"username"`
	Size     int64  `json:"size"`
}

type DiskUsage struct {
	TotalFiles int64       `json:"totalFiles"`
	TotalDirs  int64       `json:"totalDirs"`
	TotalSize  int64       `json:"totalSize"`
	TotalSpace int64       `json:"totalSpace"`
	FreeSpace  int64       `json:"freeSpace"`
	Quota      int64       `json:"quota,omitempty"` // caller's quota (0 = unlimited)
	PerUser    []UserUsage `json:"perUser"`
}

func NewDiskHandler(root string) *DiskHandler {
	return &DiskHandler{root: root}
}

func (h *DiskHandler) Usage(w http.ResponseWriter, r *http.Request) {
	usage := DiskUsage{}

	// Get filesystem total/free space (platform-specific; see disk_unix.go).
	usage.TotalSpace, usage.FreeSpace = fsSpace(h.root)

	// Non-admins must only see their own home folder's usage — the per-user
	// breakdown would otherwise leak every user's folder name and size.
	role := middleware.GetRole(r)
	ownDir := strings.Trim(middleware.GetHomeFolder(r), "/")
	if h.quotaOf != nil {
		usage.Quota = h.quotaOf(middleware.GetUsername(r))
	}

	// Calculate per-user sizes from top-level directories
	userSizes := make(map[string]int64)

	entries, err := os.ReadDir(h.root)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if role != "admin" && entry.Name() != ownDir {
				continue
			}
			dirPath := filepath.Join(h.root, entry.Name())
			var dirSize int64
			filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					usage.TotalDirs++
				} else {
					usage.TotalFiles++
					dirSize += info.Size()
					usage.TotalSize += info.Size()
				}
				return nil
			})
			userSizes[entry.Name()] = dirSize
		}
	}

	for name, size := range userSizes {
		usage.PerUser = append(usage.PerUser, UserUsage{Username: name, Size: size})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(usage)
}

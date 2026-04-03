package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type DiskHandler struct {
	root string
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
	PerUser    []UserUsage `json:"perUser"`
}

func NewDiskHandler(root string) *DiskHandler {
	return &DiskHandler{root: root}
}

func (h *DiskHandler) Usage(w http.ResponseWriter, r *http.Request) {
	usage := DiskUsage{}

	// Get filesystem total/free space
	var stat syscall.Statfs_t
	if err := syscall.Statfs(h.root, &stat); err == nil {
		usage.TotalSpace = int64(stat.Blocks) * int64(stat.Bsize)
		usage.FreeSpace = int64(stat.Bavail) * int64(stat.Bsize)
	}

	// Calculate per-user sizes from top-level directories
	userSizes := make(map[string]int64)

	entries, err := os.ReadDir(h.root)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
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

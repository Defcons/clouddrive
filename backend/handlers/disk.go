package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

type DiskHandler struct {
	root string
}

type DiskUsage struct {
	TotalFiles   int   `json:"totalFiles"`
	TotalDirs    int   `json:"totalDirs"`
	TotalSize    int64 `json:"totalSize"`
	StorageRoot  string `json:"storageRoot"`
}

func NewDiskHandler(root string) *DiskHandler {
	return &DiskHandler{root: root}
}

func (h *DiskHandler) Usage(w http.ResponseWriter, r *http.Request) {
	usage := DiskUsage{StorageRoot: h.root}

	filepath.Walk(h.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			usage.TotalDirs++
		} else {
			usage.TotalFiles++
			usage.TotalSize += info.Size()
		}
		return nil
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(usage)
}

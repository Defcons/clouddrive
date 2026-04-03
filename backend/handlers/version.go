package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

var serverStartTime = time.Now().UnixMilli()

type VersionHandler struct{}

func NewVersionHandler() *VersionHandler {
	return &VersionHandler{}
}

func (h *VersionHandler) Info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"startTime": serverStartTime,
	})
}

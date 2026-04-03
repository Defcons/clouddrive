package services

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type AuditEntry struct {
	Timestamp string `json:"timestamp"`
	Action    string `json:"action"`
	Username  string `json:"username"`
	IP        string `json:"ip"`
	Detail    string `json:"detail"`
}

type AuditLogger struct {
	filePath string
	file     *os.File
	mu       sync.Mutex
}

func NewAuditLogger(storageRoot string) *AuditLogger {
	logPath := filepath.Join(storageRoot, ".audit.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return &AuditLogger{filePath: logPath}
	}
	return &AuditLogger{filePath: logPath, file: f}
}

func (a *AuditLogger) Log(action, username, ip, detail string) {
	if a.file == nil {
		return
	}

	entry := AuditEntry{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Action:    action,
		Username:  username,
		IP:        ip,
		Detail:    detail,
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	a.file.Write(data)
	a.file.WriteString("\n")
}

// GetRecent reads the last N audit entries (newest first)
func (a *AuditLogger) GetRecent(limit int) []AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()

	f, err := os.Open(a.filePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var all []AuditEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var entry AuditEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err == nil {
			all = append(all, entry)
		}
	}

	// Reverse to newest first
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}

	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}

	return all
}

func (a *AuditLogger) Close() {
	if a.file != nil {
		a.file.Close()
	}
}

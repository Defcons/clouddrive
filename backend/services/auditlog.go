package services

import (
	"bufio"
	"encoding/json"
	"log/slog"
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

// maxAuditBytes caps the active audit log; when exceeded it rotates to
// <path>.1 (one generation kept) so both the file and the GetRecent read stay
// bounded instead of growing without limit.
var maxAuditBytes int64 = 5 << 20 // 5 MiB (var, not const, so tests can shrink it)

type AuditLogger struct {
	filePath string
	file     *os.File
	size     int64
	mu       sync.Mutex
}

func NewAuditLogger(storageRoot string) *AuditLogger {
	logPath := filepath.Join(storageRoot, ".audit.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		// Audit is security-relevant — never disable it silently.
		slog.Error("AUDIT LOGGING DISABLED: cannot open audit file; security events will NOT be recorded", "path", logPath, "err", err)
		return &AuditLogger{filePath: logPath}
	}
	var size int64
	if info, serr := f.Stat(); serr == nil {
		size = info.Size()
	}
	return &AuditLogger{filePath: logPath, file: f, size: size}
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
		slog.Warn("audit marshal failed", "err", err)
		return
	}
	n, err := a.file.Write(append(data, '\n'))
	if err != nil {
		// Audit is critical — surface failures visibly so ops notices.
		slog.Error("audit write failed", "err", err, "action", action, "user", username)
		return
	}
	a.size += int64(n)
	if a.size >= maxAuditBytes {
		a.rotateLocked()
	}
}

// rotateLocked renames the active log to <path>.1 (replacing any previous
// generation) and reopens a fresh file. Caller holds a.mu. On failure it keeps
// logging to the current file rather than losing audit coverage.
func (a *AuditLogger) rotateLocked() {
	if a.file == nil {
		return
	}
	if err := a.file.Close(); err != nil {
		slog.Warn("audit rotate: close failed", "err", err)
		return
	}
	if err := os.Rename(a.filePath, a.filePath+".1"); err != nil {
		slog.Warn("audit rotate: rename failed", "err", err)
	}
	f, err := os.OpenFile(a.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		slog.Error("AUDIT LOGGING DISABLED after rotation: cannot reopen file", "path", a.filePath, "err", err)
		a.file = nil
		return
	}
	a.file = f
	a.size = 0
}

// GetRecent reads the last N audit entries (newest first). It reads the rotated
// generation (<path>.1) before the active file so "recent" is correct even right
// after a rotation, when the active file is still near-empty. Both files are
// size-bounded by rotation, so the read stays bounded.
func (a *AuditLogger) GetRecent(limit int) []AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()

	var all []AuditEntry
	for _, p := range []string{a.filePath + ".1", a.filePath} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			var entry AuditEntry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err == nil {
				all = append(all, entry)
			}
		}
		f.Close()
	}

	// Reverse to newest first.
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

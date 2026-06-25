package services

import (
	"sort"
	"sync"
	"time"
)

// Session is a server-side record of an issued login, enabling per-session
// listing and revocation (JWTs alone are stateless).
type Session struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	CreatedAt int64  `json:"createdAt"`
	LastSeen  int64  `json:"lastSeen"`
	UserAgent string `json:"userAgent"`
	IP        string `json:"ip"`
}

type SessionStore struct {
	filePath string
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewSessionStore(storageRoot string) *SessionStore {
	s := &SessionStore{
		filePath: storageRoot + "/.sessions.json",
		sessions: make(map[string]*Session),
	}
	loadJSONFile(s.filePath, &s.sessions)
	if s.sessions == nil {
		s.sessions = make(map[string]*Session)
	}
	return s
}

func (s *SessionStore) save() {
	_ = saveJSONFile(s.filePath, s.sessions)
}

// Create registers a new session and returns its id (to embed as the JWT jti).
func (s *SessionStore) Create(username, userAgent, ip string, nowMillis int64) string {
	id := randHex(16)
	s.mu.Lock()
	s.sessions[id] = &Session{
		ID:        id,
		Username:  username,
		CreatedAt: nowMillis,
		LastSeen:  nowMillis,
		UserAgent: userAgent,
		IP:        ip,
	}
	s.save()
	s.mu.Unlock()
	return id
}

// IsValid reports whether a session id is still active.
func (s *SessionStore) IsValid(id string) bool {
	if id == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.sessions[id]
	return ok
}

// Touch updates last-seen/IP in memory (not persisted per-request to avoid
// disk churn; approximate last-seen is fine and resets on restart).
func (s *SessionStore) Touch(id, ip string, nowMillis int64) {
	s.mu.Lock()
	if sess, ok := s.sessions[id]; ok {
		sess.LastSeen = nowMillis
		if ip != "" {
			sess.IP = ip
		}
	}
	s.mu.Unlock()
}

// List returns a user's active sessions, most-recently-seen first.
func (s *SessionStore) List(username string) []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Session
	for _, sess := range s.sessions {
		if sess.Username == username {
			out = append(out, *sess)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen > out[j].LastSeen })
	return out
}

// Revoke removes a session if it belongs to username (or caller is admin).
func (s *SessionStore) Revoke(id, username, role string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return false
	}
	if role != "admin" && sess.Username != username {
		return false
	}
	delete(s.sessions, id)
	s.save()
	return true
}

// RevokeAllForUser drops every session for a user (e.g. on password change).
func (s *SessionStore) RevokeAllForUser(username string) {
	s.mu.Lock()
	changed := false
	for id, sess := range s.sessions {
		if sess.Username == username {
			delete(s.sessions, id)
			changed = true
		}
	}
	if changed {
		s.save()
	}
	s.mu.Unlock()
}

// PruneExpired drops sessions older than maxAge (called at startup).
func (s *SessionStore) PruneExpired(maxAge time.Duration, nowMillis int64) {
	cutoff := nowMillis - maxAge.Milliseconds()
	s.mu.Lock()
	changed := false
	for id, sess := range s.sessions {
		if sess.CreatedAt < cutoff {
			delete(s.sessions, id)
			changed = true
		}
	}
	if changed {
		s.save()
	}
	s.mu.Unlock()
}

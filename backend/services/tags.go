package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type TagStore struct {
	filePath string
	tags     map[string][]string // path -> list of tag names
	mu       sync.RWMutex
}

func NewTagStore(storageRoot string) *TagStore {
	store := &TagStore{
		filePath: filepath.Join(storageRoot, ".tags.json"),
		tags:     make(map[string][]string),
	}
	store.load()
	return store
}

func (s *TagStore) load() {
	loadJSONFile(s.filePath, &s.tags)
}

func (s *TagStore) save() error {
	data, err := json.MarshalIndent(s.tags, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.filePath)
}

func (s *TagStore) GetTags(path string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tags[path]
}

func (s *TagStore) SetTags(path string, tags []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(tags) == 0 {
		delete(s.tags, path)
	} else {
		s.tags[path] = tags
	}
	return s.save()
}

func (s *TagStore) AddTag(path, tag string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := s.tags[path]
	for _, t := range existing {
		if t == tag {
			return nil // already has this tag
		}
	}
	s.tags[path] = append(existing, tag)
	return s.save()
}

func (s *TagStore) RemoveTag(path, tag string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := s.tags[path]
	var updated []string
	for _, t := range existing {
		if t != tag {
			updated = append(updated, t)
		}
	}
	if len(updated) == 0 {
		delete(s.tags, path)
	} else {
		s.tags[path] = updated
	}
	return s.save()
}

// GetAllTagged returns all paths that have any tags (for enriching file listings)
func (s *TagStore) GetAllTagged() map[string][]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copy := make(map[string][]string, len(s.tags))
	for k, v := range s.tags {
		copy[k] = v
	}
	return copy
}

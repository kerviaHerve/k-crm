package sessions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Store struct {
	path string
	mu   sync.Mutex
	ids  map[string]int64
}

type file struct {
	IDs map[string]int64 `json:"ids"`
}

func Memory() *Store {
	return &Store{ids: map[string]int64{}}
}

func Open(dir string) (*Store, error) {
	s := &Store{path: filepath.Join(dir, "sessions.json"), ids: map[string]int64{}}
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		return s, nil
	}
	if f.IDs != nil {
		s.ids = f.IDs
	}
	s.prune(time.Now())
	return s, nil
}

func (s *Store) Valid(id string) bool {
	if s == nil || id == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.ids[id]
	if !ok {
		return false
	}
	if time.Now().Unix() >= exp {
		delete(s.ids, id)
		_ = s.flushLocked()
		return false
	}
	return true
}

func (s *Store) Put(id string, ttl time.Duration) error {
	if s == nil || id == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ids == nil {
		s.ids = map[string]int64{}
	}
	s.ids[id] = time.Now().Add(ttl).Unix()
	s.pruneLocked(time.Now())
	return s.flushLocked()
}

func (s *Store) Delete(id string) error {
	if s == nil || id == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ids, id)
	return s.flushLocked()
}

func (s *Store) prune(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(now)
	_ = s.flushLocked()
}

func (s *Store) pruneLocked(now time.Time) {
	unix := now.Unix()
	for id, exp := range s.ids {
		if exp <= unix {
			delete(s.ids, id)
		}
	}
}

func (s *Store) flushLocked() error {
	if s.path == "" {
		return nil
	}
	if s.ids == nil {
		s.ids = map[string]int64{}
	}
	b, err := json.Marshal(file{IDs: s.ids})
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

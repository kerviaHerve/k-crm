package agentkeys

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/ids"
)

type Key struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"`
	Hash      string `json:"hash,omitempty"`
	CreatedAt string `json:"created_at"`
	LastUsed  string `json:"last_used,omitempty"`
}

type Store struct {
	path string
	mu   sync.Mutex
	keys []Key
}

func Open(dir, legacyToken string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "agent-keys.json")}
	b, err := os.ReadFile(s.path)
	if err == nil {
		if err := json.Unmarshal(b, &s.keys); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	legacy := strings.TrimSpace(legacyToken)
	if legacy != "" && !s.hasHash(hashToken(legacy)) {
		s.keys = append(s.keys, Key{
			ID:        ids.New(),
			Name:      "install",
			Prefix:    prefixOf(legacy),
			Hash:      hashToken(legacy),
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) hasHash(h string) bool {
	for _, k := range s.keys {
		if k.Hash == h {
			return true
		}
	}
	return false
}

func (s *Store) Check(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	want := hashToken(token)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, k := range s.keys {
		if subtle.ConstantTimeCompare([]byte(k.Hash), []byte(want)) == 1 {
			s.keys[i].LastUsed = time.Now().UTC().Format(time.RFC3339)
			_ = s.saveLocked()
			return true
		}
	}
	return false
}

func (s *Store) List() []Key {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Key, 0, len(s.keys))
	for _, k := range s.keys {
		k.Hash = ""
		out = append(out, k)
	}
	return out
}

func (s *Store) Create(name string) (Key, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Key{}, "", fmt.Errorf("name required")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Key{}, "", err
	}
	plain := hex.EncodeToString(raw)
	k := Key{
		ID:        ids.New(),
		Name:      name,
		Prefix:    prefixOf(plain),
		Hash:      hashToken(plain),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys = append(s.keys, k)
	if err := s.saveLocked(); err != nil {
		return Key{}, "", err
	}
	pub := k
	pub.Hash = ""
	return pub, plain, nil
}

func (s *Store) Revoke(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.keys[:0]
	found := false
	for _, k := range s.keys {
		if k.ID == id {
			found = true
			continue
		}
		next = append(next, k)
	}
	if !found {
		return fmt.Errorf("not found")
	}
	if len(next) == 0 {
		return fmt.Errorf("keep at least one agent key")
	}
	s.keys = next
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.keys, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func prefixOf(token string) string {
	if len(token) < 8 {
		return token
	}
	return token[:8]
}

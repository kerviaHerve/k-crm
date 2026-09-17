package store

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
)

func (s *Store) personDir(id string) (string, error) {
	if id == "" || strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return "", fmt.Errorf("bad id")
	}
	return filepath.Join(filepath.Dir(s.path), "people", id), nil
}

func (s *Store) PersonAvatarPath(id string) string {
	dir, err := s.personDir(id)
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "avatar")
}

func (s *Store) HasPersonAvatar(id string) bool {
	path := s.PersonAvatarPath(id)
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func (s *Store) SetPersonAvatar(id string, raw []byte) error {
	if _, err := s.GetPerson(id); err != nil {
		return err
	}
	if len(raw) == 0 || len(raw) > 800*1024 {
		return fmt.Errorf("unreadable image")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "jpeg" && format != "png") {
		return fmt.Errorf("jpeg or png only")
	}
	if cfg.Width > 2048 || cfg.Height > 2048 {
		return fmt.Errorf("image too large")
	}
	dir, err := s.personDir(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, "avatar")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return err
	}
	ctype := "image/jpeg"
	if format == "png" {
		ctype = "image/png"
	}
	return os.WriteFile(path+".type", []byte(ctype), 0o600)
}

func (s *Store) ClearPersonAvatar(id string) error {
	if _, err := s.GetPerson(id); err != nil {
		return err
	}
	path := s.PersonAvatarPath(id)
	_ = os.Remove(path)
	_ = os.Remove(path + ".type")
	return nil
}

func (s *Store) ReadPersonAvatar(id string) ([]byte, string, error) {
	if !s.HasPersonAvatar(id) {
		return nil, "", ErrNotFound
	}
	path := s.PersonAvatarPath(id)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	ctype := "image/jpeg"
	if t, err := os.ReadFile(path + ".type"); err == nil {
		ctype = string(bytes.TrimSpace(t))
	}
	return raw, ctype, nil
}

package setup

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Config struct {
	Listen       string `json:"listen"`
	Domain       string `json:"domain"`
	HTTPS        bool   `json:"https"`
	User         string `json:"user"`
	PasswordHash string `json:"password_hash"`
	TOTPSecret   string `json:"totp_secret"`
	Done         bool   `json:"done"`
	TokenShown   bool     `json:"token_shown"`
	Activities   []string `json:"activities"`
}

type File struct {
	path string
	mu   sync.Mutex
	cfg  Config
}

type Pending struct {
	Listen   string
	Domain   string
	HTTPS    bool
	User     string
	PassHash string
	TOTP     string
}

func Open(dir string) (*File, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "config.json")
	f := &File{path: path}
	b, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(b, &f.cfg); err != nil {
			return nil, err
		}
		return f, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	return f, nil
}

func (f *File) Done() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cfg.Done
}

func (f *File) Public() Config {
	f.mu.Lock()
	defer f.mu.Unlock()
	return Config{Listen: f.cfg.Listen, Domain: f.cfg.Domain, HTTPS: f.cfg.HTTPS, User: f.cfg.User, Done: f.cfg.Done, TokenShown: f.cfg.TokenShown, Activities: append([]string(nil), f.cfg.Activities...)}
}

func (f *File) User() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cfg.User
}

func ValidateListen(listen string) error {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Errorf("listen must be ip:port")
	}
	if host == "0.0.0.0" || host == "*" || host == "" || host == "::" {
		return fmt.Errorf("refusing wildcard bind %q", listen)
	}
	if ip := net.ParseIP(host); ip == nil {
		return fmt.Errorf("listen host must be an IP")
	}
	return nil
}

func HashPassword(pw string) (string, error) {
	if len(pw) < 10 {
		return "", fmt.Errorf("password must be at least 10 characters")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (f *File) Commit(p Pending) error {
	if err := ValidateListen(p.Listen); err != nil {
		return err
	}
	if strings.TrimSpace(p.User) == "" || p.PassHash == "" || p.TOTP == "" {
		return fmt.Errorf("incomplete wizard")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cfg = Config{
		Listen: p.Listen, Domain: strings.TrimSpace(p.Domain), HTTPS: p.HTTPS,
		User: strings.TrimSpace(p.User), PasswordHash: p.PassHash, TOTPSecret: p.TOTP,
		Done: true, TokenShown: false,
	}
	return f.saveLocked()
}

func (f *File) Verify(user, password, totp string, now time.Time) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.cfg.Done || user != f.cfg.User {
		return false
	}
	if bcrypt.CompareHashAndPassword([]byte(f.cfg.PasswordHash), []byte(password)) != nil {
		return false
	}
	return VerifyTOTP(f.cfg.TOTPSecret, totp, now)
}

func (f *File) MarkTokenShown() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cfg.TokenShown = true
	return f.saveLocked()
}

func (f *File) TokenAlreadyShown() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cfg.TokenShown
}

func (f *File) ChangePassword(current, next, totp string, now time.Time) error {
	f.mu.Lock()
	hash, secret := f.cfg.PasswordHash, f.cfg.TOTPSecret
	done := f.cfg.Done
	f.mu.Unlock()
	if !done {
		return fmt.Errorf("wizard required")
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(current)) != nil || !VerifyTOTP(secret, totp, now) {
		return fmt.Errorf("invalid credentials")
	}
	newHash, err := HashPassword(next)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cfg.PasswordHash = newHash
	return f.saveLocked()
}

func (f *File) ReplaceTOTP(secret string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return fmt.Errorf("totp required")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cfg.TOTPSecret = secret
	return f.saveLocked()
}

func NormalizeActivities(in []string) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if len([]rune(name)) > 40 {
			return nil, fmt.Errorf("activite trop longue")
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	if len(out) > 24 {
		return nil, fmt.Errorf("trop d'activites")
	}
	return out, nil
}

func (f *File) SetActivities(in []string) error {
	out, err := NormalizeActivities(in)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cfg.Activities = out
	return f.saveLocked()
}

func (f *File) VerifyPassword(password string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.cfg.Done {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(f.cfg.PasswordHash), []byte(password)) == nil
}

func (f *File) saveLocked() error {
	b, err := json.MarshalIndent(f.cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}

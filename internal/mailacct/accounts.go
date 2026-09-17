package mailacct

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"brain.op3.ch/sun221/k-crm/internal/ids"
)

const (
	KindIMAP = "imap"
	KindSMTP = "smtp"
	SecTLS   = "tls"
	SecStart = "starttls"
	SecNone  = "none"
)

var (
	ErrSMTPExists = errors.New("un seul compte d'envoi")
	ErrNotFound   = errors.New("compte introuvable")
	ErrPassword   = errors.New("mot de passe requis")
)

type Account struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Security    string `json:"security"`
	Username    string `json:"username"`
	Folder      string `json:"folder,omitempty"`
	From        string `json:"from,omitempty"`
	HasPassword bool   `json:"has_password"`
	CreatedAt   string `json:"created_at"`
	LastOK      string `json:"last_ok,omitempty"`
	LastError   string `json:"last_error,omitempty"`
}

type record struct {
	Account
	Password string `json:"password"`
}

type Input struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Host     string `json:"host"`
	Port     Port   `json:"port"`
	Security string `json:"security"`
	Username string `json:"username"`
	Password string `json:"password"`
	Folder   string `json:"folder"`
	From     string `json:"from"`
}

type Store struct {
	path string
	mu   sync.Mutex
	all  []record
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "mail-accounts.json")}
	b, err := os.ReadFile(s.path)
	if err == nil {
		if err := json.Unmarshal(b, &s.all); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

func (s *Store) List() []Account {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Account, 0, len(s.all))
	for _, r := range s.all {
		out = append(out, public(r))
	}
	return out
}

func (s *Store) Create(in Input) (Account, error) {
	rec, err := normalize(in, true)
	if err != nil {
		return Account{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec.Kind == KindSMTP {
		for _, r := range s.all {
			if r.Kind == KindSMTP {
				return Account{}, ErrSMTPExists
			}
		}
	}
	rec.ID = ids.New()
	rec.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	s.all = append(s.all, rec)
	if err := s.saveLocked(); err != nil {
		return Account{}, err
	}
	return public(rec), nil
}

func (s *Store) Update(id string, in Input) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexLocked(id)
	if i < 0 {
		return Account{}, ErrNotFound
	}
	cur := s.all[i]
	if in.Kind == "" {
		in.Kind = cur.Kind
	}
	if in.Kind != cur.Kind {
		return Account{}, fmt.Errorf("kind locked")
	}
	if strings.TrimSpace(in.Password) == "" {
		in.Password = cur.Password
	}
	rec, err := normalize(in, false)
	if err != nil {
		return Account{}, err
	}
	rec.ID = cur.ID
	rec.CreatedAt = cur.CreatedAt
	rec.LastOK = cur.LastOK
	rec.LastError = cur.LastError
	s.all[i] = rec
	if err := s.saveLocked(); err != nil {
		return Account{}, err
	}
	return public(rec), nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexLocked(id)
	if i < 0 {
		return ErrNotFound
	}
	s.all = append(s.all[:i], s.all[i+1:]...)
	return s.saveLocked()
}

func (s *Store) Test(id string) (Account, error) {
	s.mu.Lock()
	rec, ok := s.recordLocked(id)
	s.mu.Unlock()
	if !ok {
		return Account{}, ErrNotFound
	}
	err := Probe(rec.Account, rec.Password)
	now := time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.indexLocked(id)
	if i < 0 {
		return Account{}, ErrNotFound
	}
	if err != nil {
		s.all[i].LastError = err.Error()
		s.all[i].LastOK = ""
	} else {
		s.all[i].LastOK = now
		s.all[i].LastError = ""
	}
	_ = s.saveLocked()
	pub := public(s.all[i])
	return pub, err
}

func (s *Store) indexLocked(id string) int {
	for i, r := range s.all {
		if r.ID == id {
			return i
		}
	}
	return -1
}

func (s *Store) recordLocked(id string) (record, bool) {
	i := s.indexLocked(id)
	if i < 0 {
		return record{}, false
	}
	return s.all[i], true
}

func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.all, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func public(r record) Account {
	a := r.Account
	a.HasPassword = r.Password != ""
	return a
}

func normalize(in Input, create bool) (record, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return record{}, fmt.Errorf("nom requis")
	}
	if len(name) > 80 {
		return record{}, fmt.Errorf("nom trop long")
	}
	kind := strings.ToLower(strings.TrimSpace(in.Kind))
	if kind != KindIMAP && kind != KindSMTP {
		return record{}, fmt.Errorf("kind imap ou smtp")
	}
	host := strings.ToLower(strings.TrimSpace(in.Host))
	if host == "" {
		return record{}, fmt.Errorf("hote requis")
	}
	if strings.Contains(host, "://") || strings.Contains(host, " ") || strings.Contains(host, ":") {
		return record{}, fmt.Errorf("hote sans schema ni port")
	}
	sec := strings.ToLower(strings.TrimSpace(in.Security))
	if sec == "" {
		sec = SecTLS
	}
	if sec != SecTLS && sec != SecStart && sec != SecNone {
		return record{}, fmt.Errorf("securite tls, starttls ou none")
	}
	user := strings.TrimSpace(in.Username)
	if user == "" {
		return record{}, fmt.Errorf("identifiant requis")
	}
	pass := in.Password
	if create && strings.TrimSpace(pass) == "" {
		return record{}, ErrPassword
	}
	port := int(in.Port)
	if port == 0 {
		port = defaultPort(kind, sec)
	}
	if port < 1 || port > 65535 {
		return record{}, fmt.Errorf("port invalide")
	}
	rec := record{
		Account: Account{
			Name:     name,
			Kind:     kind,
			Host:     host,
			Port:     port,
			Security: sec,
			Username: user,
		},
		Password: pass,
	}
	if kind == KindIMAP {
		folder := strings.TrimSpace(in.Folder)
		if folder == "" {
			folder = "INBOX"
		}
		rec.Folder = folder
	} else {
		from := strings.TrimSpace(in.From)
		if from == "" || !strings.Contains(from, "@") || strings.ContainsFunc(from, unicode.IsSpace) {
			return record{}, fmt.Errorf("adresse d'envoi requise")
		}
		rec.From = from
	}
	return rec, nil
}

func defaultPort(kind, sec string) int {
	if kind == KindIMAP {
		if sec == SecTLS {
			return 993
		}
		return 143
	}
	if sec == SecTLS {
		return 465
	}
	return 587
}

type Port int

func (p *Port) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == `""` || s == "" {
		*p = 0
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err == nil {
		*p = Port(n)
		return nil
	}
	var t string
	if err := json.Unmarshal(b, &t); err != nil {
		return fmt.Errorf("port invalide")
	}
	t = strings.TrimSpace(t)
	if t == "" {
		*p = 0
		return nil
	}
	n, err := strconv.Atoi(t)
	if err != nil {
		return fmt.Errorf("port invalide")
	}
	*p = Port(n)
	return nil
}

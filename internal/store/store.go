package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"brain.op3.ch/sun221/k-crm/internal/ids"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrNotProspect = errors.New("not a prospect")
	ErrDuplicate   = errors.New("already in the book")
)

type Store struct {
	db   *sql.DB
	path string
}

type Person struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Org       string `json:"org"`
	Pole      string `json:"pole"`
	World     string `json:"world"`
	Lead      string `json:"lead"`
	LeadState string `json:"lead_state"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Why       string `json:"why,omitempty"`
	Due       string `json:"due,omitempty"`
	Channel   string `json:"channel,omitempty"`
}

type Note struct {
	ID        string `json:"id"`
	PersonID  string `json:"person_id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

type Fiche struct {
	Person
	Notes []Note `json:"notes"`
}

type AujourdHui struct {
	GeneratedAt string   `json:"generated_at"`
	Overdue     []Person `json:"overdue"`
	Today       []Person `json:"today"`
	Orphans     []Person `json:"orphans"`
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := applyStagedRestore(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS people (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  org TEXT NOT NULL DEFAULT '',
  pole TEXT NOT NULL DEFAULT '',
  world TEXT NOT NULL CHECK (world IN ('prospect','client')),
  lead TEXT NOT NULL DEFAULT '',
  lead_state TEXT NOT NULL DEFAULT 'nouveau',
  phone TEXT NOT NULL DEFAULT '',
  email TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS relances (
  id TEXT PRIMARY KEY,
  person_id TEXT NOT NULL REFERENCES people(id),
  due TEXT,
  channel TEXT NOT NULL DEFAULT 'tel',
  why TEXT NOT NULL DEFAULT '',
  open INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS notes (
  id TEXT PRIMARY KEY,
  person_id TEXT NOT NULL REFERENCES people(id),
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS relances_open_due ON relances(open, due);
CREATE TABLE IF NOT EXISTS ingest (
  message_id TEXT PRIMARY KEY,
  person_id TEXT NOT NULL REFERENCES people(id),
  note_id TEXT NOT NULL,
  from_email TEXT NOT NULL DEFAULT '',
  subject TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
`)
	return err
}

func (s *Store) CreateProspect(p Person, due, why, channel string) (Person, error) {
	p.Name = strings.TrimSpace(p.Name)
	due = strings.TrimSpace(due)
	why = strings.TrimSpace(why)
	if p.Name == "" {
		return Person{}, fmt.Errorf("name required")
	}
	if due == "" {
		return Person{}, fmt.Errorf("prospect requires a relance date")
	}
	if _, err := time.Parse("2006-01-02", due); err != nil {
		return Person{}, fmt.Errorf("due must be YYYY-MM-DD")
	}
	if why == "" {
		return Person{}, fmt.Errorf("relance why required")
	}
	if dup, ok, err := s.FindDuplicate(p); err != nil {
		return Person{}, err
	} else if ok {
		return Person{}, fmt.Errorf("%w (%s)", ErrDuplicate, dup.Name)
	}
	p.ID = ids.New()
	p.World = "prospect"
	if p.LeadState == "" {
		p.LeadState = "nouveau"
	}
	if channel == "" {
		channel = "tel"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.Begin()
	if err != nil {
		return Person{}, err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.Exec(
		`INSERT INTO people (id,name,org,pole,world,lead,lead_state,phone,email,created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.Name, p.Org, p.Pole, p.World, p.Lead, p.LeadState, p.Phone, p.Email, now,
	)
	if err != nil {
		return Person{}, err
	}
	_, err = tx.Exec(
		`INSERT INTO relances (id,person_id,due,channel,why,open,created_at) VALUES (?,?,?,?,?,1,?)`,
		ids.New(), p.ID, due, channel, why, now,
	)
	if err != nil {
		return Person{}, err
	}
	if err := tx.Commit(); err != nil {
		return Person{}, err
	}
	p.Due, p.Why, p.Channel = due, why, channel
	return p, nil
}

func (s *Store) GetPerson(id string) (Person, error) {
	row := s.db.QueryRow(`
SELECT id,name,org,pole,world,lead,lead_state,phone,email,'','',''
FROM people WHERE id=?`, id)
	var p Person
	err := row.Scan(&p.ID, &p.Name, &p.Org, &p.Pole, &p.World, &p.Lead, &p.LeadState, &p.Phone, &p.Email, &p.Due, &p.Why, &p.Channel)
	if errors.Is(err, sql.ErrNoRows) {
		return Person{}, ErrNotFound
	}
	return p, err
}

func (s *Store) UpdatePerson(id string, in Person) (Person, error) {
	cur, err := s.GetPerson(id)
	if err != nil {
		return Person{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Person{}, fmt.Errorf("name required")
	}
	_, err = s.db.Exec(`UPDATE people SET name=?, org=?, pole=?, lead=?, phone=?, email=? WHERE id=?`,
		name, strings.TrimSpace(in.Org), strings.TrimSpace(in.Pole), strings.TrimSpace(in.Lead),
		strings.TrimSpace(in.Phone), strings.TrimSpace(in.Email), id)
	if err != nil {
		return Person{}, err
	}
	cur.Name, cur.Org, cur.Pole, cur.Lead, cur.Phone, cur.Email = name, strings.TrimSpace(in.Org), strings.TrimSpace(in.Pole), strings.TrimSpace(in.Lead), strings.TrimSpace(in.Phone), strings.TrimSpace(in.Email)
	return cur, nil
}

func (s *Store) Notes(personID string) ([]Note, error) {
	rows, err := s.db.Query(`SELECT id,person_id,title,body,created_at FROM notes WHERE person_id=? ORDER BY created_at DESC`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Note{}
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.PersonID, &n.Title, &n.Body, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) Fiche(id string) (Fiche, error) {
	p, err := s.GetPerson(id)
	if err != nil {
		return Fiche{}, err
	}
	notes, err := s.Notes(id)
	if err != nil {
		return Fiche{}, err
	}
	return Fiche{Person: p, Notes: notes}, nil
}

func (s *Store) AddNote(personID, title, body string) (Note, error) {
	body = strings.TrimSpace(body)
	title = strings.TrimSpace(title)
	if body == "" {
		return Note{}, fmt.Errorf("note body required")
	}
	if title == "" {
		title = "Note"
	}
	if _, err := s.GetPerson(personID); err != nil {
		return Note{}, err
	}
	n := Note{ID: ids.New(), PersonID: personID, Title: title, Body: body, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	_, err := s.db.Exec(`INSERT INTO notes (id,person_id,title,body,created_at) VALUES (?,?,?,?,?)`,
		n.ID, n.PersonID, n.Title, n.Body, n.CreatedAt)
	return n, err
}

func (s *Store) ValidateLead(id string) (Person, error) {
	p, err := s.GetPerson(id)
	if err != nil {
		return Person{}, err
	}
	if p.World != "prospect" {
		return Person{}, ErrNotProspect
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.Begin()
	if err != nil {
		return Person{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE people SET world='client', lead_state='validé' WHERE id=?`, id); err != nil {
		return Person{}, err
	}
	if _, err := tx.Exec(`INSERT INTO notes (id,person_id,title,body,created_at) VALUES (?,?,?,?,?)`,
		ids.New(), id, "Lead validé", "Acte explicite. Entrée en clientèle.", now); err != nil {
		return Person{}, err
	}
	if err := tx.Commit(); err != nil {
		return Person{}, err
	}
	p.World = "client"
	p.LeadState = "validé"
	return p, nil
}

func (s *Store) AujourdHui(now time.Time) (AujourdHui, error) {
	day := now.UTC().Format("2006-01-02")
	out := AujourdHui{GeneratedAt: now.UTC().Format(time.RFC3339)}
	var err error
	out.Overdue, err = s.relancePeople(`r.open=1 AND r.due IS NOT NULL AND r.due <> '' AND r.due < ?`, day)
	if err != nil {
		return AujourdHui{}, err
	}
	out.Today, err = s.relancePeople(`r.open=1 AND r.due = ?`, day)
	if err != nil {
		return AujourdHui{}, err
	}
	out.Orphans, err = s.queryPeople(`
		SELECT p.id,p.name,p.org,p.pole,p.world,p.lead,p.lead_state,p.phone,p.email,'','',''
		FROM people p
		WHERE p.world='prospect'
		  AND p.lead_state <> 'perdu'
		  AND NOT EXISTS (SELECT 1 FROM relances r WHERE r.person_id=p.id AND r.open=1)
		ORDER BY p.name`)
	if err != nil {
		return AujourdHui{}, err
	}
	return out, nil
}

func (s *Store) relancePeople(where string, arg string) ([]Person, error) {
	q := `
SELECT p.id,p.name,p.org,p.pole,p.world,p.lead,p.lead_state,p.phone,p.email,
       COALESCE(r.due,''), COALESCE(r.why,''), COALESCE(r.channel,'')
FROM relances r
JOIN people p ON p.id=r.person_id
WHERE ` + where + `
ORDER BY r.due, p.name`
	return s.queryPeople(q, arg)
}

func (s *Store) queryPeople(q string, args ...any) ([]Person, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Person
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.ID, &p.Name, &p.Org, &p.Pole, &p.World, &p.Lead, &p.LeadState, &p.Phone, &p.Email, &p.Due, &p.Why, &p.Channel); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if out == nil {
		out = []Person{}
	}
	return out, rows.Err()
}

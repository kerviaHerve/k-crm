package store

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestUpdatePersonWhyHeatChannel(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.CreateProspect(Person{Name: "Nora", Heat: "chaud"}, "2026-09-20", "appel", "telephone")
	if err != nil {
		t.Fatal(err)
	}
	if p.Heat != "chaud" || p.Channel != "tel" || p.Why != "appel" {
		t.Fatalf("create %+v", p)
	}
	got, err := s.UpdatePerson(p.ID, Person{Name: "Nora Rey", Why: "devis signé", Channel: "autres", Heat: "tiede"})
	if err != nil {
		t.Fatal(err)
	}
	if got.World != "prospect" || got.Why != "devis signé" || got.Channel != "autres" || got.Heat != "tiede" {
		t.Fatalf("got %+v", got)
	}
	if _, err := s.UpdatePerson(p.ID, Person{Name: "Nora Rey", Heat: "brulant"}); err == nil {
		t.Fatal("expected bad heat")
	}
}

func TestDeleteRelanceRequiresNextOnProspect(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.CreateProspect(Person{Name: "Lea"}, "2026-09-20", "appel", "tel")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteRelance(p.ID, "", "", "", "tel"); err == nil {
		t.Fatal("expected next relance required")
	}
	got, err := s.DeleteRelance(p.ID, "", "2026-09-25", "nouveau motif", "autres")
	if err != nil {
		t.Fatal(err)
	}
	if got.Due != "2026-09-25" || got.Why != "nouveau motif" || got.Channel != "autres" {
		t.Fatalf("got %+v", got)
	}
}

func TestDeleteRelanceClientNoNext(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.CreateProspect(Person{Name: "Marc"}, "2026-09-20", "appel", "tel")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ValidateLead(p.ID); err != nil {
		t.Fatal(err)
	}
	got, err := s.DeleteRelance(p.ID, "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.World != "client" || got.Due != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestReactivateLost(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.CreateProspect(Person{Name: "Paul"}, "2026-09-20", "appel", "tel")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reactivate(p.ID, "2026-09-22", "on reprend", "tel"); !errors.Is(err, ErrNotLost) {
		t.Fatalf("want ErrNotLost, got %v", err)
	}
	if _, err := s.MarkLost(p.ID, "plus de budget"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Reactivate(p.ID, "2026-09-22", "on reprend", "autres")
	if err != nil {
		t.Fatal(err)
	}
	if got.World != "prospect" || got.LeadState != "en cours" || got.Due != "2026-09-22" || got.Why != "on reprend" || got.Channel != "autres" {
		t.Fatalf("got %+v", got)
	}
	if _, err := s.ValidateLead(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reactivate(p.ID, "2026-09-23", "nope", "tel"); !errors.Is(err, ErrNotProspect) {
		t.Fatalf("client reactivate: %v", err)
	}
}

func TestMigrateAddsHeatFromOldSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
CREATE TABLE people (
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
CREATE TABLE relances (
  id TEXT PRIMARY KEY,
  person_id TEXT NOT NULL REFERENCES people(id),
  due TEXT,
  channel TEXT NOT NULL DEFAULT 'tel',
  why TEXT NOT NULL DEFAULT '',
  open INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL
);
CREATE TABLE notes (
  id TEXT PRIMARY KEY,
  person_id TEXT NOT NULL REFERENCES people(id),
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  created_at TEXT NOT NULL
);
INSERT INTO people (id,name,org,pole,world,lead,lead_state,phone,email,created_at)
VALUES ('p1','Old','','','prospect','','nouveau','','','2026-09-01T00:00:00Z');
INSERT INTO relances (id,person_id,due,channel,why,open,created_at)
VALUES ('r1','p1','2026-09-20','tel','appel',1,'2026-09-01T00:00:00Z');
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.GetPerson("p1")
	if err != nil {
		t.Fatal(err)
	}
	if p.Heat != "" || p.Why != "appel" || p.Name != "Old" {
		t.Fatalf("migrated %+v", p)
	}
	got, err := s.UpdatePerson("p1", Person{Name: "Old", Heat: "froid"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Heat != "froid" {
		t.Fatalf("heat %+v", got)
	}
}

func TestPersonAvatarRoundTrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.CreateProspect(Person{Name: "Ada"}, "2026-09-20", "appel", "tel")
	if err != nil {
		t.Fatal(err)
	}
	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
		0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x00, 0x03, 0x00, 0x01, 0x00, 0x05, 0xfe, 0xd4, 0xef, 0x00, 0x00,
		0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
	if err := s.SetPersonAvatar(p.ID, png); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetPerson(p.ID)
	if err != nil || !got.HasAvatar {
		t.Fatalf("has avatar %+v %v", got, err)
	}
	raw, ctype, err := s.ReadPersonAvatar(p.ID)
	if err != nil || ctype != "image/png" || len(raw) == 0 {
		t.Fatalf("read %s %v", ctype, err)
	}
	if err := s.ClearPersonAvatar(p.ID); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetPerson(p.ID)
	if err != nil || got.HasAvatar {
		t.Fatalf("cleared %+v %v", got, err)
	}
	if _, err := os.Stat(s.PersonAvatarPath(p.ID)); !os.IsNotExist(err) {
		t.Fatalf("file still there: %v", err)
	}
}

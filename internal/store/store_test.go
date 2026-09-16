package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAujourdHuiSplitsOverdueTodayOrphans(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	day := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	if _, err := s.CreateProspect(Person{Name: "Lea", Org: "Nord", Pole: "Exonik"}, "2026-09-15", "devis", "tel"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProspect(Person{Name: "Marc", Org: "Rolle", Pole: "Kervia"}, "2026-09-16", "cadrage", "mail"); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreateProspect(Person{Name: "Paul", Org: "Rey", Pole: "OP3"}, "2026-09-16", "audit", "tel")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE relances SET open=0 WHERE person_id=?`, p.ID); err != nil {
		t.Fatal(err)
	}

	got, err := s.AujourdHui(day)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Overdue) != 1 || got.Overdue[0].Name != "Lea" {
		t.Fatalf("overdue=%v", got.Overdue)
	}
	if len(got.Today) != 1 || got.Today[0].Name != "Marc" {
		t.Fatalf("today=%v", got.Today)
	}
	if len(got.Orphans) != 1 || got.Orphans[0].Name != "Paul" {
		t.Fatalf("orphans=%v", got.Orphans)
	}
}

func TestCreateProspectRejectsEmptyDue(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.CreateProspect(Person{Name: "X"}, "", "why", "tel"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := s.CreateProspect(Person{Name: "X"}, "2026-09-16", "", "tel"); err == nil {
		t.Fatal("expected why error")
	}
}

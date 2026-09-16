package store

import (
	"path/filepath"
	"strings"
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

func TestValidateLeadAndNote(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.CreateProspect(Person{Name: "Lea"}, "2026-09-16", "devis", "tel")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddNote(p.ID, "Appel", "Elle veut un devis."); err != nil {
		t.Fatal(err)
	}
	got, err := s.ValidateLead(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.World != "client" || got.LeadState != "validé" {
		t.Fatalf("got %+v", got)
	}
	if _, err := s.ValidateLead(p.ID); err != ErrNotProspect {
		t.Fatalf("expected ErrNotProspect, got %v", err)
	}
	fiche, err := s.Fiche(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fiche.Notes) != 2 {
		t.Fatalf("notes=%d", len(fiche.Notes))
	}
	if _, err := s.ValidateLead("missing"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRelanceLostAndSearch(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.CreateProspect(Person{Name: "Nora"}, "2026-09-16", "appel", "tel")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteRelance(p.ID, "", "x", "tel"); err == nil {
		t.Fatal("prospect complete without next should fail")
	}
	if _, err := s.CompleteRelance(p.ID, "2026-09-20", "suite devis", "mail"); err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	ah, err := s.AujourdHui(day)
	if err != nil {
		t.Fatal(err)
	}
	if len(ah.Overdue) != 0 || len(ah.Today) != 0 {
		t.Fatalf("after complete overdue=%d today=%d", len(ah.Overdue), len(ah.Today))
	}
	hits, err := s.Search("Nora")
	if err != nil || len(hits) != 1 {
		t.Fatalf("search=%v err=%v", hits, err)
	}
	if _, err := s.MarkLost(p.ID, "pas de budget"); err != nil {
		t.Fatal(err)
	}
	ah, err = s.AujourdHui(day)
	if err != nil {
		t.Fatal(err)
	}
	if len(ah.Orphans) != 0 {
		t.Fatalf("lost should not be orphan: %v", ah.Orphans)
	}
	st, err := s.Snapshot(day)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.People) != 1 || st.People[0].When != "none" || st.People[0].LeadState != "perdu" {
		t.Fatalf("snapshot=%+v", st.People)
	}
}

func TestUpdatePersonKeepsWorld(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.CreateProspect(Person{Name: "Nora"}, "2026-09-20", "appel", "tel")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.UpdatePerson(p.ID, Person{Name: "Nora Rey", Org: "Rey", Pole: "OP3", World: "client", LeadState: "validé", Lead: "audit"})
	if err != nil {
		t.Fatal(err)
	}
	if got.World != "prospect" || got.Name != "Nora Rey" || got.Pole != "OP3" {
		t.Fatalf("got %+v", got)
	}
	w, err := s.WaitOnThem(p.ID, "2026-09-22", "")
	if err != nil {
		t.Fatal(err)
	}
	if w.LeadState != "en attente" || w.Channel != "attente" {
		t.Fatalf("wait %+v", w)
	}
}

func TestImportProspectsNeverCreatesClient(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	csv := "name,org,pole,due,why,world\n" +
		"Ada,Nord,Exonik,2026-09-20,appel,client\n" +
		"Bad,Nord,Exonik,,missing why,prospect\n"
	got, err := s.ImportProspects(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if got.Created != 1 || got.Skipped != 1 {
		t.Fatalf("import %+v", got)
	}
	hits, err := s.Search("Ada")
	if err != nil || len(hits) != 1 || hits[0].World != "prospect" {
		t.Fatalf("ada=%v err=%v", hits, err)
	}
}

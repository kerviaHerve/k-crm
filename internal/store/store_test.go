package store

import (
	"errors"
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

func TestSearchHitsNotesAndAccents(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.CreateProspect(Person{Name: "Léa Morel", Org: "Atelier Nord", Phone: "021 555 12 12"}, "2026-09-20", "devis", "tel")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddNote(p.ID, "Appel", "Ils rappellent pour le cuivre"); err != nil {
		t.Fatal(err)
	}
	byName, err := s.SearchHits("lea")
	if err != nil || len(byName) != 1 || byName[0].Match != "nom" {
		t.Fatalf("accent %+v err=%v", byName, err)
	}
	byNote, err := s.SearchHits("cuivre")
	if err != nil || len(byNote) != 1 || byNote[0].Match != "note" {
		t.Fatalf("note %+v err=%v", byNote, err)
	}
	byPhone, err := s.SearchHits("021555")
	if err != nil || len(byPhone) != 1 {
		t.Fatalf("phone %+v err=%v", byPhone, err)
	}
}

func TestDuplicateEmailSkipped(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.CreateProspect(Person{Name: "Ada", Email: "ada@nord.ch"}, "2026-09-20", "appel", "mail"); err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateProspect(Person{Name: "Ada Lovelace", Email: "ADA@nord.ch"}, "2026-09-21", "suite", "mail")
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("want duplicate, got %v", err)
	}
	csv := "name,org,due,why,email\nAda,Nord,2026-09-22,rappel,ada@nord.ch\n"
	got, err := s.ImportProspects(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if got.Created != 0 || got.Skipped != 1 {
		t.Fatalf("import dup %+v", got)
	}
}

func TestBackupAndStagedRestore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "k-crm.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProspect(Person{Name: "Nora"}, "2026-09-20", "cadrage", "tel"); err != nil {
		t.Fatal(err)
	}
	info, err := s.Backup()
	if err != nil {
		t.Fatal(err)
	}
	if !ValidBackupName(info.Name) {
		t.Fatalf("name %s", info.Name)
	}
	if _, err := s.CreateProspect(Person{Name: "Later"}, "2026-09-21", "suite", "tel"); err != nil {
		t.Fatal(err)
	}
	if err := s.StageRestore(info.Name); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	hits, err := s2.Search("Nora")
	if err != nil || len(hits) != 1 {
		t.Fatalf("nora after restore %+v err=%v", hits, err)
	}
	later, err := s2.Search("Later")
	if err != nil || len(later) != 0 {
		t.Fatalf("later should be gone %+v err=%v", later, err)
	}
	lost, err := s2.ListLost()
	if err != nil || len(lost) != 0 {
		t.Fatalf("lost %+v err=%v", lost, err)
	}
}

func TestIngestFilesKnownAddressSkipsNoiseAndUnknown(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.CreateProspect(Person{Name: "Lea Morel", Org: "Nord", Email: "lea@nord.example"}, "2026-09-20", "devis", "mail")
	if err != nil {
		t.Fatal(err)
	}
	mail := "From: Lea Morel <lea@nord.example>\r\nSubject: Devis site\r\nMessage-ID: <ing1@nord.example>\r\n\r\nBonjour, on avance sur le devis de la page d'accueil.\r\n"
	got, err := s.IngestRaw(mail)
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != "filed" || got.PersonID != p.ID {
		t.Fatalf("filed %+v", got)
	}
	dup, err := s.IngestRaw(mail)
	if err != nil {
		t.Fatal(err)
	}
	if dup.Action != "duplicate" {
		t.Fatalf("dup %+v", dup)
	}
	skip, err := s.IngestRaw("From: noreply@shop.example\r\nSubject: Order\r\n\r\nThanks for your order, it will ship tomorrow morning.\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if skip.Action != "skipped" || skip.Reason != "noreply" {
		t.Fatalf("skip %+v", skip)
	}
	miss, err := s.IngestRaw("From: inconnu@ailleurs.example\r\nSubject: Hello there friend\r\n\r\nThis is a real looking message from nobody we know yet.\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if miss.Action != "unmatched" {
		t.Fatalf("unmatched %+v", miss)
	}
	fiche, err := s.Fiche(p.ID)
	if err != nil || len(fiche.Notes) != 1 {
		t.Fatalf("notes %+v err=%v", fiche.Notes, err)
	}
}

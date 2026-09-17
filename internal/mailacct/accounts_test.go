package mailacct

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateListHidesPassword(t *testing.T) {
	s := openT(t)
	a, err := s.Create(Input{
		Name: "Net6", Kind: KindIMAP, Host: "mail.net6.ch", Port: 993,
		Security: SecTLS, Username: "crm@net6.ch", Password: "secret-value", Folder: "CRM",
	})
	if err != nil {
		t.Fatal(err)
	}
	if a.HasPassword != true || a.Folder != "CRM" || a.Port != 993 {
		t.Fatalf("%+v", a)
	}
	b, err := json.Marshal(s.List())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret-value") || strings.Contains(string(b), `"password"`) {
		t.Fatalf("password leaked: %s", b)
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(s.path), "mail-accounts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "secret-value") {
		t.Fatal("password not persisted")
	}
	st, err := os.Stat(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", st.Mode().Perm())
	}
}

func TestOneSMTP(t *testing.T) {
	s := openT(t)
	if _, err := s.Create(Input{
		Name: "Envoi", Kind: KindSMTP, Host: "smtp.net6.ch",
		Security: SecStart, Username: "herve@net6.ch", Password: "x", From: "herve@net6.ch",
	}); err != nil {
		t.Fatal(err)
	}
	_, err := s.Create(Input{
		Name: "Autre", Kind: KindSMTP, Host: "smtp.example.com",
		Security: SecStart, Username: "a@b.c", Password: "y", From: "a@b.c",
	})
	if err != ErrSMTPExists {
		t.Fatalf("got %v", err)
	}
	if _, err := s.Create(Input{
		Name: "Collecte", Kind: KindIMAP, Host: "imap.net6.ch",
		Security: SecTLS, Username: "crm@net6.ch", Password: "z",
	}); err != nil {
		t.Fatal(err)
	}
	if n := len(s.List()); n != 2 {
		t.Fatalf("n=%d", n)
	}
}

func TestUpdateKeepsPassword(t *testing.T) {
	s := openT(t)
	a, err := s.Create(Input{
		Name: "Boite", Kind: KindIMAP, Host: "imap.example.com",
		Security: SecTLS, Username: "u", Password: "keep-me",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Update(a.ID, Input{
		Name: "Boite CRM", Kind: KindIMAP, Host: "imap.example.com",
		Security: SecTLS, Username: "u", Folder: "KCRM",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Folder != "KCRM" || got.Name != "Boite CRM" {
		t.Fatalf("%+v", got)
	}
	raw, _ := os.ReadFile(s.path)
	if !strings.Contains(string(raw), "keep-me") {
		t.Fatal("password dropped")
	}
}

func TestRejectBadHost(t *testing.T) {
	s := openT(t)
	_, err := s.Create(Input{
		Name: "x", Kind: KindIMAP, Host: "imaps://mail.example.com:993",
		Username: "u", Password: "p",
	})
	if err == nil {
		t.Fatal("accepted scheme")
	}
}

func TestDeleteMissing(t *testing.T) {
	s := openT(t)
	if err := s.Delete("nope"); err != ErrNotFound {
		t.Fatalf("got %v", err)
	}
}

func openT(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

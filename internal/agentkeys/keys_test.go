package agentkeys

import (
	"path/filepath"
	"testing"
)

func TestCreateCheckRevoke(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir, "legacytokenvalue0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Check("legacytokenvalue0123456789abcdef") {
		t.Fatal("legacy not imported")
	}
	k, plain, err := s.Create("rita")
	if err != nil {
		t.Fatal(err)
	}
	if plain == "" || k.Prefix == "" || k.Hash != "" {
		t.Fatalf("key %+v plain empty=%v", k, plain == "")
	}
	if !s.Check(plain) {
		t.Fatal("new key rejected")
	}
	if s.Check("nope") {
		t.Fatal("garbage accepted")
	}
	if err := s.Revoke(k.ID); err != nil {
		t.Fatal(err)
	}
	if s.Check(plain) {
		t.Fatal("revoked still valid")
	}
	listed := s.List()
	if len(listed) != 1 || listed[0].Name != "install" {
		t.Fatalf("list %+v", listed)
	}
}

func TestRefuseLastRevoke(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "k"), "onlykey0123456789abcdef01234567")
	if err != nil {
		t.Fatal(err)
	}
	id := s.List()[0].ID
	if err := s.Revoke(id); err == nil {
		t.Fatal("revoked last key")
	}
}

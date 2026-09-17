package sessions

import (
	"path/filepath"
	"testing"
	"time"
)

func TestPutValidSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Put("abc", time.Hour); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !s2.Valid("abc") {
		t.Fatal("session lost after reopen")
	}
	if s2.Valid("nope") {
		t.Fatal("unknown id accepted")
	}
	if err := s2.Delete("abc"); err != nil {
		t.Fatal(err)
	}
	s3, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s3.Valid("abc") {
		t.Fatal("deleted session still valid")
	}
	_ = filepath.Join(dir, "sessions.json")
}

func TestExpiredRejected(t *testing.T) {
	s := Memory()
	if err := s.Put("old", -time.Second); err != nil {
		t.Fatal(err)
	}
	if s.Valid("old") {
		t.Fatal("expired session accepted")
	}
}

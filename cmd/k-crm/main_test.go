package main

import (
	"testing"

	"brain.op3.ch/sun221/k-crm/internal/setup"
)

func TestRejectWildcard(t *testing.T) {
	if err := rejectWildcard("0.0.0.0:80"); err == nil {
		t.Fatal("expected reject")
	}
	if err := rejectWildcard("127.0.0.1:8740"); err != nil {
		t.Fatal(err)
	}
}

func TestPickListenUsesConfigAfterWizard(t *testing.T) {
	dir := t.TempDir()
	f, err := setup.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := pickListen("127.0.0.1:8740", false, f); got != "127.0.0.1:8740" {
		t.Fatalf("fresh install got %s", got)
	}
	hash, err := setup.HashPassword("correcthorse")
	if err != nil {
		t.Fatal(err)
	}
	sec, err := setup.NewTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Commit(setup.Pending{
		Listen: "100.100.73.244:8795", User: "herve", PassHash: hash, TOTP: sec,
	}); err != nil {
		t.Fatal(err)
	}
	if got := pickListen("127.0.0.1:8740", false, f); got != "100.100.73.244:8795" {
		t.Fatalf("config listen got %s", got)
	}
	if got := pickListen("127.0.0.1:9000", true, f); got != "127.0.0.1:9000" {
		t.Fatalf("explicit flag got %s", got)
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"brain.op3.ch/sun221/k-crm/internal/setup"
	"brain.op3.ch/sun221/k-crm/internal/version"
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

func TestVersionIsBeta(t *testing.T) {
	if !strings.Contains(version.Number, "beta") {
		t.Fatalf("version %s is not beta", version.Number)
	}
}

func TestVersionDoesNotCreateData(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nope")
	if err := run([]string{"version", "-data", dir}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("version created data dir")
	}
}

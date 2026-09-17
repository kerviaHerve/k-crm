package setup

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTOTPRoundTrip(t *testing.T) {
	sec, err := NewTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	code, err := Code(sec, now)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyTOTP(sec, code, now) {
		t.Fatal("expected match")
	}
	if VerifyTOTP(sec, "000000", now) {
		t.Fatal("expected reject")
	}
}

func TestWizardCommitAndLogin(t *testing.T) {
	dir := t.TempDir()
	f, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f.Done() {
		t.Fatal("new file should not be done")
	}
	if err := ValidateListen("0.0.0.0:80"); err == nil {
		t.Fatal("wildcard")
	}
	hash, err := HashPassword("correcthorse")
	if err != nil {
		t.Fatal(err)
	}
	sec, err := NewTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Commit(Pending{
		Listen: "127.0.0.1:8740", Domain: "k-crm.local", User: "herve",
		PassHash: hash, TOTP: sec,
	}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	code, err := Code(sec, now)
	if err != nil {
		t.Fatal(err)
	}
	if !f.Verify("herve", "correcthorse", code, now) {
		t.Fatal("login should pass")
	}
	if f.Verify("herve", "wrong-password-long", code, now) {
		t.Fatal("bad password")
	}
	if f.Public().PasswordHash != "" || f.Public().TOTPSecret != "" {
		t.Fatal("public leaked secrets")
	}
	if err := f.ChangePassword("correcthorse", "newhorsebattery", code, now); err != nil {
		t.Fatal(err)
	}
	if f.Verify("herve", "correcthorse", code, now) {
		t.Fatal("old password still valid")
	}
	if !f.Verify("herve", "newhorsebattery", code, now) {
		t.Fatal("new password rejected")
	}
}

func TestTOTPQR(t *testing.T) {
	otpauth, qr, err := TOTPQR("herve", "MFRGGZDFMZTWQ2LK")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(otpauth, "otpauth://totp/K-CRM:herve?") {
		t.Fatalf("otpauth=%s", otpauth)
	}
	if !strings.Contains(otpauth, "secret=MFRGGZDFMZTWQ2LK") || !strings.Contains(otpauth, "issuer=K-CRM") {
		t.Fatalf("otpauth missing fields %s", otpauth)
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(qr, prefix) {
		t.Fatalf("qr prefix %s", qr[:min(40, len(qr))])
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(qr, prefix))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 100 || string(raw[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("not a png len=%d", len(raw))
	}
}

package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"brain.op3.ch/sun221/k-crm/internal/update"
)

func TestUpdateGetNotGit(t *testing.T) {
	s := &Server{DataDir: t.TempDir(), Update: &update.Env{Root: t.TempDir()}}
	req := httptest.NewRequest(http.MethodGet, "/ui/api/update", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d %s", rec.Code, rec.Body.String())
	}
	var st update.Status
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.Reason != "not_git" || st.CanApply {
		t.Fatalf("%+v", st)
	}
	if st.Version == "" {
		t.Fatal("missing version")
	}
}

func TestUpdateApplyRefusesDirty(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	remote := t.TempDir()
	root := t.TempDir()
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
			"GIT_TERMINAL_PROMPT=0",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %s\n%s", args, err, out)
		}
	}
	run(remote, "init", "--bare")
	run(root, "init", "-b", "main")
	if err := os.MkdirAll(filepath.Join(root, "cmd/k-crm"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.26.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd/k-crm/main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(root, "add", ".")
	run(root, "commit", "-m", "init")
	run(root, "remote", "add", "origin", remote)
	run(root, "push", "-u", "origin", "main")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Server{Update: &update.Env{
		Root:        root,
		AllowOrigin: remote,
		Backup:      func() error { return nil },
		Build:       func(context.Context, string, string) error { return nil },
	}}
	req := httptest.NewRequest(http.MethodPost, "/ui/api/update", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "modifiés") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestHealthzReportsVersion(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	(&Server{}).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"version"`) {
		t.Fatalf("body %s", rec.Body.String())
	}
}

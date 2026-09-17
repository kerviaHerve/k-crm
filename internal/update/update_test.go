package update

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalOrigin(t *testing.T) {
	cases := map[string]string{
		"git@github.com:kerviaHerve/k-crm.git":         "github.com/kerviaherve/k-crm",
		"https://github.com/kerviaHerve/k-crm.git":     "github.com/kerviaherve/k-crm",
		"https://github.com/kerviaHerve/k-crm":         "github.com/kerviaherve/k-crm",
		"ssh://git@github.com/kerviaHerve/k-crm.git":   "github.com/kerviaherve/k-crm",
		"https://x:y@github.com/kerviaHerve/k-crm.git": "github.com/kerviaherve/k-crm",
	}
	for in, want := range cases {
		if got := CanonicalOrigin(in); got != want {
			t.Errorf("%s -> %s, want %s", in, got, want)
		}
	}
}

func TestOriginAllowed(t *testing.T) {
	e := &Env{}
	if !e.originAllowed("git@github.com:kerviaHerve/k-crm.git") {
		t.Fatal("official origin should pass")
	}
	if e.originAllowed("https://github.com/other/k-crm.git") {
		t.Fatal("foreign origin should fail")
	}
	e.AllowOrigin = "/tmp/bare.git"
	if !e.originAllowed("/tmp/bare.git") {
		t.Fatal("test origin should pass")
	}
}

func gitAvail(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
}

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_TERMINAL_PROMPT=0",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %s\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setupPair(t *testing.T) (root, remote string) {
	t.Helper()
	gitAvail(t)
	remote = t.TempDir()
	gitCmd(t, remote, "init", "--bare", "-b", "main")
	root = t.TempDir()
	gitCmd(t, root, "init", "-b", "main")
	write(t, root, "go.mod", "module brain.op3.ch/sun221/k-crm\n\ngo 1.26.0\n")
	write(t, root, "cmd/k-crm/main.go", "package main\nfunc main() {}\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-m", "init")
	gitCmd(t, root, "remote", "add", "origin", remote)
	gitCmd(t, root, "push", "-u", "origin", "HEAD:main")
	return root, remote
}

func pushExtra(t *testing.T, remote string) string {
	t.Helper()
	work := filepath.Join(t.TempDir(), "src")
	gitCmd(t, filepath.Dir(work), "clone", remote, work)
	write(t, work, "extra.txt", "n\n")
	gitCmd(t, work, "add", "extra.txt")
	gitCmd(t, work, "commit", "-m", "extra")
	gitCmd(t, work, "push", "origin", "HEAD:main")
	return strings.TrimSpace(gitCmd(t, work, "rev-parse", "HEAD"))
}

func TestCheckAvailableAndApply(t *testing.T) {
	root, remote := setupPair(t)
	want := pushExtra(t, remote)
	bin := filepath.Join(root, "k-crm")
	if err := os.WriteFile(bin, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	backed := 0
	e := &Env{
		Root:        root,
		Bin:         bin,
		AllowOrigin: remote,
		Backup:      func() error { backed++; return nil },
		Build: func(_ context.Context, _, out string) error {
			return os.WriteFile(out, []byte("new"), 0o755)
		},
	}
	st := e.Check(context.Background(), true)
	if !st.Available || !st.CanApply || st.Reason != "available" {
		t.Fatalf("%+v", st)
	}
	res, err := e.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if backed != 1 {
		t.Fatalf("backup %d", backed)
	}
	if res.Head != want {
		t.Fatalf("head %s want %s", res.Head, want)
	}
	got, err := os.ReadFile(bin)
	if err != nil || string(got) != "new" {
		t.Fatalf("bin %q %v", got, err)
	}
	prev, err := os.ReadFile(bin + ".prev")
	if err != nil || string(prev) != "old" {
		t.Fatalf("prev %q %v", prev, err)
	}
	st = e.Check(context.Background(), true)
	if st.Available || st.Reason != "up_to_date" {
		t.Fatalf("after apply %+v", st)
	}
}

func TestCheckDirty(t *testing.T) {
	root, remote := setupPair(t)
	write(t, root, "go.mod", "module dirty\n")
	e := &Env{Root: root, AllowOrigin: remote}
	st := e.Check(context.Background(), false)
	if st.Reason != "dirty" || st.CanApply {
		t.Fatalf("%+v", st)
	}
}

func TestCheckBadOrigin(t *testing.T) {
	root, _ := setupPair(t)
	e := &Env{Root: root}
	st := e.Check(context.Background(), false)
	if st.Reason != "bad_origin" || st.CanApply {
		t.Fatalf("%+v", st)
	}
}

func TestCheckAhead(t *testing.T) {
	root, remote := setupPair(t)
	write(t, root, "local.txt", "x\n")
	gitCmd(t, root, "add", "local.txt")
	gitCmd(t, root, "commit", "-m", "local")
	e := &Env{Root: root, AllowOrigin: remote}
	st := e.Check(context.Background(), false)
	if st.Reason != "ahead" || st.CanApply {
		t.Fatalf("%+v", st)
	}
}

func TestCheckNotGit(t *testing.T) {
	e := &Env{Root: t.TempDir()}
	st := e.Check(context.Background(), false)
	if st.Reason != "not_git" {
		t.Fatalf("%+v", st)
	}
}

func TestDiscoverFromDataDir(t *testing.T) {
	root, _ := setupPair(t)
	data := filepath.Join(root, "data")
	if err := os.Mkdir(data, 0o700); err != nil {
		t.Fatal(err)
	}
	e := Discover(data)
	if e.Root != root {
		t.Fatalf("root %s want %s", e.Root, root)
	}
}

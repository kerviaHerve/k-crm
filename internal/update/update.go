package update

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/version"
)

const (
	allowedOrigin = "github.com/kerviaherve/k-crm"
	githubHTTPS   = "https://github.com/kerviaHerve/k-crm.git"
	fetchRefspec  = "+refs/heads/main:refs/remotes/origin/main"
)

var (
	ErrBusy = errors.New("mise à jour déjà en cours")
	goVerRE = regexp.MustCompile(`go(\d+)\.(\d+)`)
)

type Status struct {
	Version   string `json:"version"`
	Revision  string `json:"revision"`
	Head      string `json:"head,omitempty"`
	Remote    string `json:"remote,omitempty"`
	Origin    string `json:"origin,omitempty"`
	Available bool   `json:"available"`
	CanApply  bool   `json:"can_apply"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Systemd   bool   `json:"systemd"`
}

type Result struct {
	Head    string `json:"head"`
	Bin     string `json:"bin"`
	Restart bool   `json:"restart"`
}

type Env struct {
	Root        string
	Bin         string
	Git         string
	Go          string
	Home        string
	AllowOrigin string
	Backup      func() error
	Build       func(ctx context.Context, root, out string) error

	mu sync.Mutex
}

func CanonicalOrigin(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")
	if u, err := url.Parse(s); err == nil && u.Scheme != "" && u.Host != "" {
		path := strings.Trim(u.Path, "/")
		path = strings.TrimSuffix(path, ".git")
		return strings.ToLower(u.Hostname() + "/" + path)
	}
	s = strings.TrimPrefix(s, "ssh://")
	s = strings.TrimPrefix(s, "git@")
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	s = strings.Replace(s, ":", "/", 1)
	s = strings.TrimSuffix(s, ".git")
	return strings.ToLower(s)
}

func (e *Env) originAllowed(raw string) bool {
	got := CanonicalOrigin(raw)
	if e.AllowOrigin != "" {
		return got == CanonicalOrigin(e.AllowOrigin)
	}
	return got == allowedOrigin
}

func isClone(dir string) bool {
	if dir == "" || dir == "/" {
		return false
	}
	for _, p := range []string{
		filepath.Join(dir, "go.mod"),
		filepath.Join(dir, "cmd", "k-crm"),
		filepath.Join(dir, ".git"),
	} {
		if _, err := os.Stat(p); err != nil {
			return false
		}
	}
	return true
}

func Discover(dataDir string) *Env {
	e := &Env{Home: os.Getenv("HOME")}
	if dataDir != "" {
		parent := filepath.Clean(filepath.Dir(dataDir))
		if isClone(parent) {
			e.Root = parent
		}
	}
	if e.Root == "" {
		if exe, err := os.Executable(); err == nil {
			if resolved, err := filepath.EvalSymlinks(exe); err == nil {
				exe = resolved
			}
			dir := filepath.Dir(exe)
			if isClone(dir) {
				e.Root = dir
				e.Bin = exe
			}
		}
	}
	if e.Root != "" && e.Bin == "" {
		e.Bin = filepath.Join(e.Root, "k-crm")
	}
	return e
}

func (e *Env) Check(ctx context.Context, fetch bool) Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.checkLocked(ctx, fetch)
}

func (e *Env) checkLocked(ctx context.Context, fetch bool) Status {
	st := Status{
		Version:  version.Number,
		Revision: version.ShortRev(),
		Systemd:  os.Getenv("KCRM_SYSTEMD") == "1",
		Reason:   "up_to_date",
		Message:  "Déjà à jour.",
	}
	if e.Root == "" || !isClone(e.Root) {
		st.Reason = "not_git"
		st.Message = "Ce n'est pas une install git GitHub."
		return st
	}
	git, err := e.gitBin()
	if err != nil {
		st.Reason = "no_git"
		st.Message = err.Error()
		return st
	}
	e.Git = git
	origin, code, err := e.gitRun(ctx, "remote", "get-url", "origin")
	if err != nil || code != 0 {
		st.Reason = "not_git"
		st.Message = "Ce n'est pas une install git GitHub."
		return st
	}
	origin = strings.TrimSpace(origin)
	st.Origin = CanonicalOrigin(origin)
	if !e.originAllowed(origin) {
		st.Reason = "bad_origin"
		st.Message = "L'origine git n'est pas github.com/kerviaHerve/k-crm."
		return st
	}
	if fetch {
		if err := e.fetch(ctx); err != nil {
			st.Reason = "fetch_failed"
			st.Message = "git fetch a échoué. " + err.Error()
			return st
		}
	}
	dirty, err := e.dirty(ctx)
	if err != nil {
		st.Reason = "not_git"
		st.Message = err.Error()
		return st
	}
	if dirty {
		st.Reason = "dirty"
		st.Message = "Fichiers locaux modifiés. Maj refusée."
		return st
	}
	head, code, err := e.gitRun(ctx, "rev-parse", "HEAD")
	if err != nil || code != 0 {
		st.Reason = "not_git"
		st.Message = "HEAD git illisible."
		return st
	}
	st.Head = strings.TrimSpace(head)
	remote, err := e.remoteSHA(ctx)
	if err != nil {
		st.Reason = "no_main"
		st.Message = "Branche origin/main absente."
		return st
	}
	st.Remote = remote
	if st.Head == st.Remote {
		return st
	}
	if e.isAncestor(ctx, st.Head, st.Remote) {
		st.Available = true
		st.CanApply = true
		st.Reason = "available"
		st.Message = "Une mise à jour est disponible."
		return st
	}
	if e.isAncestor(ctx, st.Remote, st.Head) {
		st.Reason = "ahead"
		st.Message = "Des commits locaux ne sont pas sur GitHub. Maj refusée."
		return st
	}
	st.Reason = "diverged"
	st.Message = "Les historiques ont divergé. Maj refusée."
	return st
}

func (e *Env) Apply(ctx context.Context) (Result, error) {
	if !e.mu.TryLock() {
		return Result{}, ErrBusy
	}
	defer e.mu.Unlock()
	st := e.checkLocked(ctx, true)
	if !st.CanApply {
		return Result{}, errors.New(st.Message)
	}
	if e.Backup != nil {
		if err := e.Backup(); err != nil {
			return Result{}, fmt.Errorf("sauvegarde: %w", err)
		}
	}
	if err := e.fetch(ctx); err != nil {
		return Result{}, fmt.Errorf("git fetch: %w", err)
	}
	st = e.checkLocked(ctx, false)
	if !st.CanApply {
		return Result{}, errors.New(st.Message)
	}
	if _, code, err := e.gitRun(ctx, "reset", "--hard", "origin/main"); err != nil || code != 0 {
		return Result{}, fmt.Errorf("git reset: %v", err)
	}
	dest := filepath.Join(e.Root, "k-crm.new")
	if err := e.build(ctx, dest); err != nil {
		return Result{}, err
	}
	if err := os.Chmod(dest, 0o755); err != nil {
		return Result{}, err
	}
	bin := e.Bin
	if bin == "" {
		bin = filepath.Join(e.Root, "k-crm")
	}
	prev := bin + ".prev"
	_ = os.Remove(prev)
	if _, err := os.Stat(bin); err == nil {
		if err := os.Rename(bin, prev); err != nil {
			_ = os.Remove(dest)
			return Result{}, fmt.Errorf("garder le binaire actuel: %w", err)
		}
	}
	if err := os.Rename(dest, bin); err != nil {
		if _, err2 := os.Stat(prev); err2 == nil {
			_ = os.Rename(prev, bin)
		}
		return Result{}, fmt.Errorf("poser le nouveau binaire: %w", err)
	}
	head, _, _ := e.gitRun(ctx, "rev-parse", "HEAD")
	return Result{
		Head:    strings.TrimSpace(head),
		Bin:     bin,
		Restart: os.Getenv("KCRM_SYSTEMD") == "1",
	}, nil
}

func (e *Env) gitBin() (string, error) {
	if e.Git != "" {
		return e.Git, nil
	}
	if p, err := exec.LookPath("git"); err == nil {
		return p, nil
	}
	for _, p := range []string{"/usr/bin/git", "/usr/local/bin/git"} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", errors.New("git introuvable")
}

func (e *Env) fetch(ctx context.Context) error {
	if e.AllowOrigin != "" {
		_, code, err := e.gitRun(ctx, "fetch", "origin")
		if err != nil || code != 0 {
			if err == nil {
				err = errors.New("fetch origin")
			}
			return err
		}
		return nil
	}
	_, code, err := e.gitRun(ctx, "fetch", githubHTTPS, fetchRefspec)
	if err != nil || code != 0 {
		if err == nil {
			err = errors.New("fetch github")
		}
		return err
	}
	return nil
}

func (e *Env) remoteSHA(ctx context.Context) (string, error) {
	out, code, err := e.gitRun(ctx, "rev-parse", "origin/main")
	if err == nil && code == 0 {
		return strings.TrimSpace(out), nil
	}
	out, code, err = e.gitRun(ctx, "rev-parse", "origin/HEAD")
	if err == nil && code == 0 {
		return strings.TrimSpace(out), nil
	}
	return "", errors.New("origin/main")
}

func (e *Env) dirty(ctx context.Context) (bool, error) {
	out, code, err := e.gitRun(ctx, "status", "--porcelain=v1", "--untracked-files=no")
	if err != nil || code != 0 {
		if err == nil {
			err = errors.New("git status")
		}
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (e *Env) isAncestor(ctx context.Context, anc, desc string) bool {
	_, code, _ := e.gitRun(ctx, "merge-base", "--is-ancestor", anc, desc)
	return code == 0
}

func (e *Env) gitRun(ctx context.Context, args ...string) (string, int, error) {
	git, err := e.gitBin()
	if err != nil {
		return "", 1, err
	}
	cmd := exec.CommandContext(ctx, git, args...)
	cmd.Dir = e.Root
	cmd.Env = gitEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err == nil {
		return stdout.String(), 0, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), ee.ExitCode(), errors.New(msg)
	}
	return stdout.String(), 1, err
}

func gitEnv() []string {
	env := os.Environ()
	out := make([]string, 0, len(env)+2)
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_TERMINAL_PROMPT=") || strings.HasPrefix(e, "GIT_ASKPASS=") {
			continue
		}
		out = append(out, e)
	}
	return append(out, "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=true")
}

func (e *Env) build(ctx context.Context, dest string) error {
	if e.Build != nil {
		return e.Build(ctx, e.Root, dest)
	}
	goBin, err := e.findGo()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, goBin, "build", "-buildvcs=true", "-o", dest, "./cmd/k-crm")
	cmd.Dir = e.Root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if dir := filepath.Dir(goBin); dir != "" && dir != "." {
		cmd.Env = prependPath(cmd.Env, dir)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("go build: %s", msg)
	}
	return nil
}

func (e *Env) findGo() (string, error) {
	if e.Go != "" {
		if goAtLeast(e.Go, 1, 26) {
			return e.Go, nil
		}
		return "", errors.New("Go 1.26+ introuvable. Relance ./install.sh une fois.")
	}
	home := e.Home
	if home == "" {
		home = os.Getenv("HOME")
	}
	var cands []string
	if p, err := exec.LookPath("go"); err == nil {
		cands = append(cands, p)
	}
	if home != "" {
		cands = append(cands,
			filepath.Join(home, ".local/share/go1.26.5/bin/go"),
			filepath.Join(home, ".local/share/go/bin/go"),
			filepath.Join(home, ".local/go/bin/go"),
		)
		if matches, err := filepath.Glob(filepath.Join(home, ".local/share/go1.26*/bin/go")); err == nil {
			cands = append(cands, matches...)
		}
	}
	cands = append(cands, "/usr/local/go/bin/go")
	seen := map[string]bool{}
	for _, c := range cands {
		if c == "" || seen[c] || strings.Contains(c, "/snap/") {
			continue
		}
		seen[c] = true
		if goAtLeast(c, 1, 26) {
			return c, nil
		}
	}
	return "", errors.New("Go 1.26+ introuvable. Relance ./install.sh une fois.")
}

func goAtLeast(bin string, major, minor int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "version").Output()
	if err != nil {
		return false
	}
	m := goVerRE.FindStringSubmatch(string(out))
	if len(m) != 3 {
		return false
	}
	var maj, min int
	_, _ = fmt.Sscanf(m[1], "%d", &maj)
	_, _ = fmt.Sscanf(m[2], "%d", &min)
	if maj > major {
		return true
	}
	return maj == major && min >= minor
}

func prependPath(env []string, dir string) []string {
	out := make([]string, 0, len(env))
	found := false
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			out = append(out, "PATH="+dir+string(os.PathListSeparator)+strings.TrimPrefix(e, "PATH="))
			found = true
			continue
		}
		out = append(out, e)
	}
	if !found {
		out = append(out, "PATH="+dir)
	}
	return out
}

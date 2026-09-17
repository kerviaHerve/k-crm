package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/sessions"
	"brain.op3.ch/sun221/k-crm/internal/setup"
	"brain.op3.ch/sun221/k-crm/internal/store"
)

func freshWizard(t *testing.T) (*Server, http.Handler, time.Time) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	cfg, err := setup.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	srv := &Server{
		Store:    st,
		Token:    "tokentestvalue",
		Setup:    cfg,
		Sessions: sessions.Memory(),
		Listen:   "127.0.0.1:8740",
		Now:      func() time.Time { return now },
	}
	return srv, srv.Routes(), now
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestWizardThenLogin(t *testing.T) {
	srv, h, now := freshWizard(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/install" {
		t.Fatalf("home before wizard code=%d loc=%s", rec.Code, rec.Header().Get("Location"))
	}
	loginEarly := httptest.NewRecorder()
	h.ServeHTTP(loginEarly, httptest.NewRequest(http.MethodGet, "/login", nil))
	if loginEarly.Code != http.StatusSeeOther || loginEarly.Header().Get("Location") != "/install" {
		t.Fatalf("login before wizard code=%d loc=%s", loginEarly.Code, loginEarly.Header().Get("Location"))
	}

	page := httptest.NewRecorder()
	h.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/install", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("install page %d", page.Code)
	}
	html := page.Body.String()
	if !strings.Contains(html, "id=\"dot6\"") || !strings.Contains(html, "gate-stage") {
		t.Fatal("install page missing six steps")
	}
	if strings.Contains(html, "100.100.73.244:8795") || strings.Contains(html, "0.0.0.0") && strings.Contains(html, `value="0.0.0.0`) {
		t.Fatal("install page hardcodes a bind")
	}

	st := doJSON(t, h, http.MethodGet, "/install/status", nil, nil)
	if st.Code != http.StatusOK {
		t.Fatalf("status %d %s", st.Code, st.Body.String())
	}
	var status map[string]any
	if err := json.Unmarshal(st.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status["done"] != false {
		t.Fatalf("status done=%v", status["done"])
	}
	if status["listen"] != "127.0.0.1:8740" {
		t.Fatalf("status listen=%v", status["listen"])
	}
	if _, ok := status["token"]; ok {
		t.Fatal("status leaked token")
	}

	badListen := doJSON(t, h, http.MethodPost, "/install/listen", map[string]string{"listen": "0.0.0.0:80"}, nil)
	if badListen.Code != http.StatusBadRequest {
		t.Fatalf("wildcard listen %d %s", badListen.Code, badListen.Body.String())
	}

	okListen := doJSON(t, h, http.MethodPost, "/install/listen", map[string]string{"listen": "127.0.0.1:8740"}, nil)
	if okListen.Code != http.StatusOK {
		t.Fatalf("listen %d %s", okListen.Code, okListen.Body.String())
	}

	short := doJSON(t, h, http.MethodPost, "/install/start", map[string]any{
		"listen": "127.0.0.1:8740", "user": "herve", "password": "short",
	}, nil)
	if short.Code != http.StatusBadRequest {
		t.Fatalf("short password %d %s", short.Code, short.Body.String())
	}

	nouser := doJSON(t, h, http.MethodPost, "/install/start", map[string]any{
		"listen": "127.0.0.1:8740", "user": "  ", "password": "correcthorse",
	}, nil)
	if nouser.Code != http.StatusBadRequest {
		t.Fatalf("empty user %d %s", nouser.Code, nouser.Body.String())
	}

	wild := doJSON(t, h, http.MethodPost, "/install/start", map[string]any{
		"listen": "0.0.0.0:80", "user": "herve", "password": "correcthorse",
	}, nil)
	if wild.Code != http.StatusBadRequest {
		t.Fatalf("start wildcard %d %s", wild.Code, wild.Body.String())
	}

	early := doJSON(t, h, http.MethodPost, "/install/confirm", map[string]string{"code": "000000"}, nil)
	if early.Code != http.StatusBadRequest {
		t.Fatalf("confirm before start %d", early.Code)
	}

	start := doJSON(t, h, http.MethodPost, "/install/start", map[string]any{
		"listen": "127.0.0.1:8740", "domain": "k-crm.local", "https": false,
		"user": "herve", "password": "correcthorse",
	}, nil)
	if start.Code != http.StatusOK {
		t.Fatalf("start %d %s", start.Code, start.Body.String())
	}
	var started map[string]string
	if err := json.Unmarshal(start.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	if started["secret"] == "" || started["otpauth"] == "" {
		t.Fatal("missing totp material")
	}

	badCode := doJSON(t, h, http.MethodPost, "/install/confirm", map[string]string{"code": "000000"}, nil)
	if badCode.Code != http.StatusUnauthorized {
		t.Fatalf("bad totp %d %s", badCode.Code, badCode.Body.String())
	}

	code, err := setup.Code(started["secret"], now)
	if err != nil {
		t.Fatal(err)
	}
	confirm := doJSON(t, h, http.MethodPost, "/install/confirm", map[string]string{"code": code}, nil)
	if confirm.Code != http.StatusOK {
		t.Fatalf("confirm %d %s", confirm.Code, confirm.Body.String())
	}
	if !strings.Contains(confirm.Body.String(), "tokentestvalue") {
		t.Fatal("token should appear once")
	}
	var confirmed map[string]any
	if err := json.Unmarshal(confirm.Body.Bytes(), &confirmed); err != nil {
		t.Fatal(err)
	}
	if confirmed["restart"] != nil {
		t.Fatal("same listen should not ask restart")
	}

	cookies := confirm.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("confirm should set session")
	}

	state := doJSON(t, h, http.MethodGet, "/ui/api/state", nil, cookies)
	if state.Code != http.StatusOK {
		t.Fatalf("state after session %d", state.Code)
	}
	naked := doJSON(t, h, http.MethodGet, "/ui/api/state", nil, nil)
	if naked.Code != http.StatusUnauthorized {
		t.Fatalf("state without session %d", naked.Code)
	}

	again := doJSON(t, h, http.MethodPost, "/install/start", map[string]any{
		"listen": "127.0.0.1:8740", "user": "herve", "password": "correcthorse",
	}, nil)
	if again.Code != http.StatusConflict {
		t.Fatalf("start after done %d %s", again.Code, again.Body.String())
	}

	repage := httptest.NewRecorder()
	h.ServeHTTP(repage, httptest.NewRequest(http.MethodGet, "/install", nil))
	if repage.Code != http.StatusSeeOther || repage.Header().Get("Location") != "/" {
		t.Fatalf("install after done code=%d loc=%s", repage.Code, repage.Header().Get("Location"))
	}

	loginPage := httptest.NewRecorder()
	h.ServeHTTP(loginPage, httptest.NewRequest(http.MethodGet, "/login", nil))
	if loginPage.Code != http.StatusOK {
		t.Fatalf("login page %d", loginPage.Code)
	}

	wrong := doJSON(t, h, http.MethodPost, "/login", map[string]string{
		"user": "herve", "password": "wrong-password-long", "code": code,
	}, nil)
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("bad login %d", wrong.Code)
	}

	okLogin := doJSON(t, h, http.MethodPost, "/login", map[string]string{
		"user": "herve", "password": "correcthorse", "code": code,
	}, nil)
	if okLogin.Code != http.StatusOK {
		t.Fatalf("login %d %s", okLogin.Code, okLogin.Body.String())
	}
	if strings.Contains(okLogin.Body.String(), "tokentestvalue") {
		t.Fatal("login leaked token")
	}
	if srv.Setup.Public().PasswordHash != "" || srv.Setup.Public().TOTPSecret != "" {
		t.Fatal("public leaked secrets")
	}
}

func TestWizardRestartWhenListenChanges(t *testing.T) {
	_, h, now := freshWizard(t)
	start := doJSON(t, h, http.MethodPost, "/install/start", map[string]any{
		"listen": "127.0.0.1:8799", "user": "herve", "password": "correcthorse",
	}, nil)
	if start.Code != http.StatusOK {
		t.Fatalf("start %d %s", start.Code, start.Body.String())
	}
	var started map[string]string
	if err := json.Unmarshal(start.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	code, err := setup.Code(started["secret"], now)
	if err != nil {
		t.Fatal(err)
	}
	confirm := doJSON(t, h, http.MethodPost, "/install/confirm", map[string]string{"code": code}, nil)
	if confirm.Code != http.StatusOK {
		t.Fatalf("confirm %d %s", confirm.Code, confirm.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(confirm.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["restart"] != true {
		t.Fatalf("expected restart flag, got %v", out)
	}
}

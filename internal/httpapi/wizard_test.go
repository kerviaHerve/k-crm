package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/setup"
	"brain.op3.ch/sun221/k-crm/internal/store"
)

func TestWizardThenLogin(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg, err := setup.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	srv := &Server{Store: st, Token: "tokentestvalue", Setup: cfg, Now: func() time.Time { return now }}
	h := srv.Routes()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/install" {
		t.Fatalf("home before wizard code=%d loc=%s", rec.Code, rec.Header().Get("Location"))
	}
	start, _ := json.Marshal(map[string]any{
		"listen": "127.0.0.1:8740", "domain": "k-crm.local", "user": "herve",
		"password": "correcthorse",
	})
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/install/start", bytes.NewReader(start)))
	if rec2.Code != http.StatusOK {
		t.Fatalf("start %d %s", rec2.Code, rec2.Body.String())
	}
	var started map[string]string
	if err := json.Unmarshal(rec2.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	code, err := setup.Code(started["secret"], now)
	if err != nil {
		t.Fatal(err)
	}
	confirm, _ := json.Marshal(map[string]string{"code": code})
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, httptest.NewRequest(http.MethodPost, "/install/confirm", bytes.NewReader(confirm)))
	if rec3.Code != http.StatusOK {
		t.Fatalf("confirm %d %s", rec3.Code, rec3.Body.String())
	}
	if !strings.Contains(rec3.Body.String(), "tokentestvalue") {
		t.Fatal("token should appear once")
	}
	cookie := rec3.Result().Cookies()
	req := httptest.NewRequest(http.MethodGet, "/ui/api/state", nil)
	for _, c := range cookie {
		req.AddCookie(c)
	}
	rec4 := httptest.NewRecorder()
	h.ServeHTTP(rec4, req)
	if rec4.Code != http.StatusOK {
		t.Fatalf("state after session %d", rec4.Code)
	}
	rec5 := httptest.NewRecorder()
	h.ServeHTTP(rec5, httptest.NewRequest(http.MethodGet, "/ui/api/state", nil))
	if rec5.Code != http.StatusUnauthorized {
		t.Fatalf("state without session %d", rec5.Code)
	}
}

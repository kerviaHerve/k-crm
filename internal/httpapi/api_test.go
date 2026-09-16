package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/store"
)

func TestAujourdHuiRequiresToken(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := &Server{Store: st, Token: "secret-test"}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/aujourd-hui", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestAujourdHuiJSON(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.CreateProspect(store.Person{Name: "Lea", Pole: "Exonik"}, "2026-09-15", "devis", "tel"); err != nil {
		t.Fatal(err)
	}
	srv := &Server{
		Store: st,
		Token: "secret-test",
		Now:   func() time.Time { return time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC) },
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/aujourd-hui", nil)
	req.Header.Set("Authorization", "Bearer secret-test")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out store.AujourdHui
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Overdue) != 1 || out.Overdue[0].Name != "Lea" {
		t.Fatalf("overdue=%v", out.Overdue)
	}
}

func TestHealthzNoAuth(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	(&Server{Store: st, Token: "x"}).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
}

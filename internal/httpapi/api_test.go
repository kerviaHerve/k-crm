package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

func TestCreateProspectHTTP(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := &Server{Store: st, Token: "secret-test"}
	body := `{"name":"Lea","pole":"Exonik","due":"2026-09-16","why":"devis"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/prospects", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret-test")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/prospects", strings.NewReader(`{"name":"X"}`))
	req2.Header.Set("Authorization", "Bearer secret-test")
	rec2 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec2.Code)
	}
}

func TestNoteAndValidateHTTP(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p, err := st.CreateProspect(store.Person{Name: "Lea"}, "2026-09-16", "devis", "tel")
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{Store: st, Token: "secret-test"}
	h := srv.Routes()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/people/"+p.ID+"/notes", strings.NewReader(`{"body":"Appel 10 min"}`))
	req.Header.Set("Authorization", "Bearer secret-test")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("note code=%d body=%s", rec.Code, rec.Body.String())
	}
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/people/"+p.ID+"/validate", nil)
	req2.Header.Set("Authorization", "Bearer secret-test")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("validate code=%d body=%s", rec2.Code, rec2.Body.String())
	}
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/people/"+p.ID+"/validate", nil)
	req3.Header.Set("Authorization", "Bearer secret-test")
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec3.Code)
	}
	req4 := httptest.NewRequest(http.MethodGet, "/api/v1/people/"+p.ID, nil)
	req4.Header.Set("Authorization", "Bearer secret-test")
	rec4 := httptest.NewRecorder()
	h.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK || !strings.Contains(rec4.Body.String(), `"world":"client"`) {
		t.Fatalf("fiche=%s", rec4.Body.String())
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

package webui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"brain.op3.ch/sun221/k-crm/internal/store"
)

func TestHomeAndCreate(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	mux := http.NewServeMux()
	Mount(mux, st, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Aujourd'hui") {
		t.Fatalf("home code=%d body=%s", rec.Code, rec.Body.String())
	}
	form := url.Values{
		"name": {"Nora Test"},
		"due":  {"2026-09-20"},
		"why":  {"premier appel"},
		"pole": {"Kervia"},
	}
	req := httptest.NewRequest(http.MethodPost, "/ui/prospects", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusSeeOther {
		t.Fatalf("create code=%d", rec2.Code)
	}
	loc := rec2.Header().Get("Location")
	if !strings.HasPrefix(loc, "/p/") {
		t.Fatalf("location=%s", loc)
	}
	rec3 := httptest.NewRecorder()
	mux.ServeHTTP(rec3, httptest.NewRequest(http.MethodGet, loc, nil))
	if rec3.Code != http.StatusOK || !strings.Contains(rec3.Body.String(), "Nora Test") {
		t.Fatalf("fiche=%s", rec3.Body.String())
	}
}

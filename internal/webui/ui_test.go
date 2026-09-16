package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHomeServesMaquette(t *testing.T) {
	mux := http.NewServeMux()
	Mount(mux, nil, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "K-CRM") || !strings.Contains(body, "Tableau de bord") {
		t.Fatalf("home missing shell")
	}
}

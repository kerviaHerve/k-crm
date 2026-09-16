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

	"brain.op3.ch/sun221/k-crm/internal/catalog"
	"brain.op3.ch/sun221/k-crm/internal/store"
)

func TestAgentCatalogAndFullAccess(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := &Server{
		Store: st,
		Token: "secret-test",
		Now:   func() time.Time { return time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC) },
	}
	h := srv.Routes()
	auth := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer secret-test")
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	rec := auth(http.MethodGet, "/api/v1/tools", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("tools %d %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, tool := range listed.Tools {
		got[tool.Name] = true
	}
	for _, name := range catalog.Names() {
		if !got[name] {
			t.Fatalf("missing tool %s", name)
		}
	}
	if len(listed.Tools) != len(catalog.Names()) {
		t.Fatalf("tools=%d catalog=%d", len(listed.Tools), len(catalog.Names()))
	}

	create := auth(http.MethodPost, "/api/v1/tools/crm_creer_personne", `{"name":"Lea Morel","pole":"Exonik","due":"2026-09-16","why":"devis"}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("create %d %s", create.Code, create.Body.String())
	}
	var person store.Person
	if err := json.Unmarshal(create.Body.Bytes(), &person); err != nil {
		t.Fatal(err)
	}
	idJSON, _ := json.Marshal(map[string]string{"id": person.ID})

	if rec := auth(http.MethodPost, "/api/v1/tools/crm_noter", `{"id":"`+person.ID+`","body":"Appel"}`); rec.Code != http.StatusCreated {
		t.Fatalf("note %d %s", rec.Code, rec.Body.String())
	}
	if rec := auth(http.MethodPost, "/api/v1/tools/crm_fiche", string(idJSON)); rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("Appel")) {
		t.Fatalf("fiche %d %s", rec.Code, rec.Body.String())
	}
	if rec := auth(http.MethodPost, "/api/v1/tools/crm_attendre", `{"id":"`+person.ID+`","due":"2026-09-20","why":"ils rappellent"}`); rec.Code != http.StatusOK {
		t.Fatalf("wait %d %s", rec.Code, rec.Body.String())
	}
	if rec := auth(http.MethodPost, "/api/v1/tools/crm_relancer", `{"id":"`+person.ID+`","due":"2026-09-22","why":"suite","channel":"tel"}`); rec.Code != http.StatusOK {
		t.Fatalf("relancer %d %s", rec.Code, rec.Body.String())
	}
	if rec := auth(http.MethodPost, "/api/v1/tools/crm_reporter", `{"id":"`+person.ID+`","due":"2026-09-25","why":"plus tard"}`); rec.Code != http.StatusOK {
		t.Fatalf("reporter %d %s", rec.Code, rec.Body.String())
	}
	if rec := auth(http.MethodPost, "/api/v1/tools/crm_modifier", `{"id":"`+person.ID+`","name":"Lea Morel","org":"Exonik SA"}`); rec.Code != http.StatusOK {
		t.Fatalf("modifier %d %s", rec.Code, rec.Body.String())
	}
	if rec := auth(http.MethodPost, "/api/v1/tools/crm_chercher", `{"q":"Lea"}`); rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("Lea")) {
		t.Fatalf("chercher %d %s", rec.Code, rec.Body.String())
	}
	csv := "name,due,why,world\nImport Client,2026-09-18,appel,client\n"
	impBody, _ := json.Marshal(map[string]string{"csv": csv})
	imp := auth(http.MethodPost, "/api/v1/tools/crm_importer", string(impBody))
	if imp.Code != http.StatusOK {
		t.Fatalf("import %d %s", imp.Code, imp.Body.String())
	}
	var imported store.ImportResult
	if err := json.Unmarshal(imp.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	if imported.Created != 1 {
		t.Fatalf("import created=%d", imported.Created)
	}
	exp := auth(http.MethodPost, "/api/v1/tools/crm_exporter", "")
	if exp.Code != http.StatusOK || !bytes.Contains(exp.Body.Bytes(), []byte("Import Client")) {
		t.Fatalf("export %d %s", exp.Code, exp.Body.String())
	}
	if rec := auth(http.MethodPost, "/api/v1/tools/crm_etat", ""); rec.Code != http.StatusOK {
		t.Fatalf("etat %d %s", rec.Code, rec.Body.String())
	}
	if rec := auth(http.MethodGet, "/api/v1/config", ""); rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"done":false`)) {
		t.Fatalf("config %d %s", rec.Code, rec.Body.String())
	}
	if rec := auth(http.MethodPost, "/api/v1/tools/crm_valider_lead", string(idJSON)); rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"world":"client"`)) {
		t.Fatalf("validate %d %s", rec.Code, rec.Body.String())
	}
	lostCreate := auth(http.MethodPost, "/api/v1/tools/crm_creer_personne", `{"name":"Paul","due":"2026-09-16","why":"essai"}`)
	var lost store.Person
	if err := json.Unmarshal(lostCreate.Body.Bytes(), &lost); err != nil {
		t.Fatal(err)
	}
	if rec := auth(http.MethodPost, "/api/v1/tools/crm_marquer_perdu", `{"id":"`+lost.ID+`","why":"plus de reponse"}`); rec.Code != http.StatusOK {
		t.Fatalf("perdu %d %s", rec.Code, rec.Body.String())
	}
}

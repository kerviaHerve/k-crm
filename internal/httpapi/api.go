package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/agentkeys"
	"brain.op3.ch/sun221/k-crm/internal/catalog"
	"brain.op3.ch/sun221/k-crm/internal/mailacct"
	"brain.op3.ch/sun221/k-crm/internal/sessions"
	"brain.op3.ch/sun221/k-crm/internal/setup"
	"brain.op3.ch/sun221/k-crm/internal/store"
	"brain.op3.ch/sun221/k-crm/internal/webui"
)

type Server struct {
	Store       *store.Store
	Token       string
	Now         func() time.Time
	Setup       *setup.File
	Keys        *agentkeys.Store
	Mail        *mailacct.Store
	Sessions    *sessions.Store
	DataDir     string
	Listen      string
	pending     *setup.Pending
	pendingTOTP string
}

type createBody struct {
	Name    string `json:"name"`
	Org     string `json:"org"`
	Pole    string `json:"pole"`
	Lead    string `json:"lead"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
	Due     string `json:"due"`
	Why     string `json:"why"`
	Channel string `json:"channel"`
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /install", s.installPage)
	mux.HandleFunc("GET /install/status", s.installStatus)
	mux.HandleFunc("POST /install/listen", s.installListen)
	mux.HandleFunc("POST /install/start", s.installStart)
	mux.HandleFunc("POST /install/confirm", s.installConfirm)
	mux.HandleFunc("GET /login", s.loginPage)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("POST /logout", s.logout)
	mux.HandleFunc("GET /api/v1/tools", s.auth(s.tools))
	mux.HandleFunc("GET /api/v1/aujourd-hui", s.auth(s.aujourdHui))
	mux.HandleFunc("POST /api/v1/tools/crm_aujourd_hui", s.auth(s.aujourdHui))
	mux.HandleFunc("POST /api/v1/prospects", s.auth(s.createProspect))
	mux.HandleFunc("POST /api/v1/tools/crm_creer_personne", s.auth(s.createProspect))
	mux.HandleFunc("GET /api/v1/people/{id}", s.auth(s.fiche))
	mux.HandleFunc("POST /api/v1/people/{id}/notes", s.auth(s.addNote))
	mux.HandleFunc("POST /api/v1/people/{id}/validate", s.auth(s.validateLead))
	mux.HandleFunc("POST /api/v1/tools/crm_noter", s.auth(s.addNoteTool))
	mux.HandleFunc("POST /api/v1/tools/crm_valider_lead", s.auth(s.validateLeadTool))
	mux.HandleFunc("GET /api/v1/state", s.auth(s.state))
	mux.HandleFunc("GET /api/v1/people", s.auth(s.search))
	mux.HandleFunc("POST /api/v1/people/{id}/lost", s.auth(s.markLost))
	mux.HandleFunc("POST /api/v1/people/{id}/relance", s.auth(s.relance))
	mux.HandleFunc("POST /api/v1/tools/crm_marquer_perdu", s.auth(s.markLostTool))
	mux.HandleFunc("POST /api/v1/tools/crm_relancer", s.auth(s.relanceCompleteTool))
	mux.HandleFunc("POST /api/v1/tools/crm_reporter", s.auth(s.relancePlanTool))
	mux.HandleFunc("POST /api/v1/tools/crm_chercher", s.auth(s.search))
	mux.HandleFunc("GET /ui/api/state", s.state)
	mux.HandleFunc("GET /ui/api/search", s.search)
	mux.HandleFunc("POST /ui/api/prospects", s.createProspect)
	mux.HandleFunc("POST /ui/api/people/{id}/notes", s.addNote)
	mux.HandleFunc("POST /ui/api/people/{id}/validate", s.validateLead)
	mux.HandleFunc("POST /ui/api/people/{id}/lost", s.markLost)
	mux.HandleFunc("POST /ui/api/people/{id}/relance", s.relance)
	mux.HandleFunc("POST /ui/api/people/{id}", s.updatePerson)
	mux.HandleFunc("POST /api/v1/people/{id}", s.auth(s.updatePerson))
	mux.HandleFunc("GET /ui/api/export.csv", s.exportCSV)
	mux.HandleFunc("GET /api/v1/export.csv", s.auth(s.exportCSV))
	mux.HandleFunc("POST /ui/api/import.csv", s.importCSV)
	mux.HandleFunc("POST /api/v1/import.csv", s.auth(s.importCSV))
	mux.HandleFunc("POST /api/v1/tools/crm_importer", s.auth(s.importTool))
	mux.HandleFunc("POST /api/v1/tools/crm_fiche", s.auth(s.ficheTool))
	mux.HandleFunc("POST /api/v1/tools/crm_attendre", s.auth(s.waitTool))
	mux.HandleFunc("POST /api/v1/tools/crm_modifier", s.auth(s.updatePersonTool))
	mux.HandleFunc("POST /api/v1/tools/crm_exporter", s.auth(s.exportTool))
	mux.HandleFunc("POST /api/v1/tools/crm_etat", s.auth(s.state))
	mux.HandleFunc("GET /api/v1/config", s.auth(s.publicConfig))
	mux.HandleFunc("GET /ui/api/settings", s.settingsMe)
	mux.HandleFunc("POST /ui/api/settings/password", s.changePassword)
	mux.HandleFunc("POST /ui/api/settings/activities", s.settingsActivities)
	mux.HandleFunc("POST /ui/api/settings/totp/start", s.totpStart)
	mux.HandleFunc("POST /ui/api/settings/totp/confirm", s.totpConfirm)
	mux.HandleFunc("GET /ui/api/avatar", s.avatarGet)
	mux.HandleFunc("POST /ui/api/avatar", s.avatarUpload)
	mux.HandleFunc("GET /ui/api/keys", s.keysList)
	mux.HandleFunc("POST /ui/api/keys", s.keysCreate)
	mux.HandleFunc("DELETE /ui/api/keys/{id}", s.keysRevoke)
	mux.HandleFunc("GET /api/v1/keys", s.auth(s.keysList))
	mux.HandleFunc("POST /api/v1/keys", s.auth(s.keysCreate))
	mux.HandleFunc("DELETE /api/v1/keys/{id}", s.auth(s.keysRevoke))
	mux.HandleFunc("POST /api/v1/tools/crm_cles_lister", s.auth(s.keysList))
	mux.HandleFunc("POST /api/v1/tools/crm_cles_creer", s.auth(s.keysCreateTool))
	mux.HandleFunc("POST /api/v1/tools/crm_cles_revoquer", s.auth(s.keysRevokeTool))
	mux.HandleFunc("GET /ui/api/people/{id}/relance.ics", s.relanceICS)
	mux.HandleFunc("GET /api/v1/perdus", s.auth(s.listLost))
	mux.HandleFunc("POST /api/v1/tools/crm_perdus", s.auth(s.listLost))
	mux.HandleFunc("POST /ui/api/ingest", s.ingest)
	mux.HandleFunc("POST /api/v1/ingest", s.auth(s.ingest))
	mux.HandleFunc("POST /api/v1/tools/crm_ingerer", s.auth(s.ingest))
	mux.HandleFunc("GET /ui/api/mail-accounts", s.mailAccounts)
	mux.HandleFunc("POST /ui/api/mail-accounts", s.mailCreate)
	mux.HandleFunc("POST /ui/api/mail-accounts/{id}", s.mailUpdate)
	mux.HandleFunc("DELETE /ui/api/mail-accounts/{id}", s.mailDelete)
	mux.HandleFunc("POST /ui/api/mail-accounts/{id}/test", s.mailTest)
	mux.HandleFunc("GET /api/v1/mail-accounts", s.auth(s.mailAccounts))
	mux.HandleFunc("POST /api/v1/mail-accounts", s.auth(s.mailCreate))
	mux.HandleFunc("POST /api/v1/mail-accounts/{id}", s.auth(s.mailUpdate))
	mux.HandleFunc("DELETE /api/v1/mail-accounts/{id}", s.auth(s.mailDelete))
	mux.HandleFunc("POST /api/v1/mail-accounts/{id}/test", s.auth(s.mailTest))
	mux.HandleFunc("POST /api/v1/tools/crm_comptes_mail_lister", s.auth(s.mailAccounts))
	mux.HandleFunc("POST /api/v1/tools/crm_comptes_mail_ajouter", s.auth(s.mailCreate))
	mux.HandleFunc("POST /api/v1/tools/crm_comptes_mail_modifier", s.auth(s.mailUpdate))
	mux.HandleFunc("POST /api/v1/tools/crm_comptes_mail_supprimer", s.auth(s.mailDelete))
	mux.HandleFunc("POST /api/v1/tools/crm_comptes_mail_tester", s.auth(s.mailTest))
	mux.HandleFunc("GET /ui/api/backups", s.backupsList)
	mux.HandleFunc("POST /ui/api/backups", s.backupsCreate)
	mux.HandleFunc("GET /ui/api/backups/{name}", s.backupsGet)
	mux.HandleFunc("POST /ui/api/backups/{name}/restore", s.backupsRestore)
	webui.Mount(mux, s.Store, s.now)
	return s.gate(mux)
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Authorization")
		if !s.bearerOK(got) {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "k-crm"})
}

func (s *Server) tools(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"tools": catalog.HTTP()})
}

func (s *Server) aujourdHui(w http.ResponseWriter, r *http.Request) {
	out, err := s.Store.AujourdHui(s.now())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createProspect(w http.ResponseWriter, r *http.Request) {
	var body createBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	p, err := s.Store.CreateProspect(store.Person{
		Name:  body.Name,
		Org:   body.Org,
		Pole:  body.Pole,
		Lead:  body.Lead,
		Phone: body.Phone,
		Email: body.Email,
	}, body.Due, body.Why, body.Channel)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func storeHTTP(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, store.ErrNotProspect) || errors.Is(err, store.ErrLost) || errors.Is(err, store.ErrDuplicate) {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	writeErr(w, http.StatusBadRequest, err.Error())
}

func (s *Server) fiche(w http.ResponseWriter, r *http.Request) {
	out, err := s.Store.Fiche(r.PathValue("id"))
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) addNote(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	n, err := s.Store.AddNote(r.PathValue("id"), body.Title, body.Body)
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func (s *Server) validateLead(w http.ResponseWriter, r *http.Request) {
	p, err := s.Store.ValidateLead(r.PathValue("id"))
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) addNoteTool(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	n, err := s.Store.AddNote(body.ID, body.Title, body.Body)
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func (s *Server) validateLeadTool(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	p, err := s.Store.ValidateLead(body.ID)
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

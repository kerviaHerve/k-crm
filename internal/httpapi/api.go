package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/store"
	"brain.op3.ch/sun221/k-crm/internal/webui"
)

type Server struct {
	Store *store.Store
	Token string
	Now   func() time.Time
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
	mux.HandleFunc("POST /api/v1/tools/crm_relancer", s.auth(s.relanceTool))
	mux.HandleFunc("POST /api/v1/tools/crm_reporter", s.auth(s.relanceTool))
	mux.HandleFunc("POST /api/v1/tools/crm_chercher", s.auth(s.search))
	mux.HandleFunc("GET /ui/api/state", s.state)
	mux.HandleFunc("GET /ui/api/search", s.search)
	mux.HandleFunc("POST /ui/api/prospects", s.createProspect)
	mux.HandleFunc("POST /ui/api/people/{id}/notes", s.addNote)
	mux.HandleFunc("POST /ui/api/people/{id}/validate", s.validateLead)
	mux.HandleFunc("POST /ui/api/people/{id}/lost", s.markLost)
	mux.HandleFunc("POST /ui/api/people/{id}/relance", s.relance)
	webui.Mount(mux, s.Store, s.now)
	return mux
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if s.Token == "" || got != s.Token {
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
	writeJSON(w, http.StatusOK, map[string]any{
		"tools": []map[string]any{
			{
				"name":        "crm_aujourd_hui",
				"description": "Relances en retard, dues aujourd'hui, et prospects sans suite.",
				"http":        []string{"GET /api/v1/aujourd-hui", "POST /api/v1/tools/crm_aujourd_hui"},
			},
			{
				"name":        "crm_creer_personne",
				"description": "Cree un prospect. Relance (due + why) obligatoire. Ne cree jamais un client.",
				"http":        []string{"POST /api/v1/prospects", "POST /api/v1/tools/crm_creer_personne"},
			},
			{
				"name":        "crm_noter",
				"description": "Ajoute une note libre sur le fil d'une personne.",
				"http":        []string{"POST /api/v1/people/{id}/notes", "POST /api/v1/tools/crm_noter"},
			},
			{
				"name":        "crm_valider_lead",
				"description": "Passe un prospect en client. Acte explicite, irreversible ici.",
				"http":        []string{"POST /api/v1/people/{id}/validate", "POST /api/v1/tools/crm_valider_lead"},
			},
		},
	})
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
	if errors.Is(err, store.ErrNotProspect) || errors.Is(err, store.ErrLost) {
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

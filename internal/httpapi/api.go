package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/store"
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

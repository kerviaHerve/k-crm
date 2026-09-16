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
	return mux
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if s.Token == "" || got != s.Token {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true,"service":"k-crm"}`))
}

func (s *Server) tools(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"tools": []map[string]any{{
			"name":        "crm_aujourd_hui",
			"description": "Relances en retard, dues aujourd'hui, et prospects sans suite.",
			"http":        []string{"GET /api/v1/aujourd-hui", "POST /api/v1/tools/crm_aujourd_hui"},
		}},
	})
}

func (s *Server) aujourdHui(w http.ResponseWriter, r *http.Request) {
	out, err := s.Store.AujourdHui(s.now())
	if err != nil {
		http.Error(w, `{"error":"store"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

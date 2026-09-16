package httpapi

import (
	"encoding/json"
	"net/http"

	"brain.op3.ch/sun221/k-crm/internal/store"
)

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	out, err := s.Store.Snapshot(s.now())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if r.Method == http.MethodPost {
		var body struct {
			Q     string `json:"q"`
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if q == "" {
			q = body.Q
		}
		if q == "" {
			q = body.Query
		}
	}
	hits, err := s.Store.Search(q)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"people": hits})
}

func (s *Server) markLost(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Why string `json:"why"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	p, err := s.Store.MarkLost(r.PathValue("id"), body.Why)
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) markLostTool(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID  string `json:"id"`
		Why string `json:"why"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	p, err := s.Store.MarkLost(body.ID, body.Why)
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) relance(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode    string `json:"mode"`
		Due     string `json:"due"`
		Why     string `json:"why"`
		Channel string `json:"channel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	s.applyRelance(w, r.PathValue("id"), body.Mode, body.Due, body.Why, body.Channel)
}

func (s *Server) relanceTool(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID      string `json:"id"`
		Mode    string `json:"mode"`
		Due     string `json:"due"`
		Why     string `json:"why"`
		Channel string `json:"channel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	s.applyRelance(w, body.ID, body.Mode, body.Due, body.Why, body.Channel)
}

func (s *Server) applyRelance(w http.ResponseWriter, id, mode, due, why, channel string) {
	var (
		p   store.Person
		err error
	)
	switch mode {
	case "complete":
		p, err = s.Store.CompleteRelance(id, due, why, channel)
	default:
		p, err = s.Store.PlanRelance(id, due, why, channel)
	}
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

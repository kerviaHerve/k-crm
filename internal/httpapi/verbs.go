package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

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
	hits, err := s.Store.SearchHits(q)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store")
		return
	}
	people := make([]store.Person, 0, len(hits))
	seen := map[string]bool{}
	for _, h := range hits {
		if seen[h.ID] {
			continue
		}
		seen[h.ID] = true
		people = append(people, h.Person)
	}
	writeJSON(w, http.StatusOK, map[string]any{"q": q, "hits": hits, "people": people})
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
	case "wait":
		p, err = s.Store.WaitOnThem(id, due, why)
	default:
		p, err = s.Store.PlanRelance(id, due, why, channel)
	}
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) updatePerson(w http.ResponseWriter, r *http.Request) {
	var body createBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	p, err := s.Store.UpdatePerson(r.PathValue("id"), store.Person{
		Name: body.Name, Org: body.Org, Pole: body.Pole, Lead: body.Lead,
		Phone: body.Phone, Email: body.Email,
	})
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) exportCSV(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"k-crm.csv\"")
	if err := s.Store.WriteCSV(w, s.now()); err != nil {
		writeErr(w, http.StatusInternalServerError, "store")
	}
}

func (s *Server) exportTool(w http.ResponseWriter, r *http.Request) {
	var buf strings.Builder
	if err := s.Store.WriteCSV(&buf, s.now()); err != nil {
		writeErr(w, http.StatusInternalServerError, "store")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"csv": buf.String()})
}

func (s *Server) importCSV(w http.ResponseWriter, r *http.Request) {
	var body io.Reader = r.Body
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid upload")
			return
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			writeErr(w, http.StatusBadRequest, "file required")
			return
		}
		defer f.Close()
		body = f
	}
	out, err := s.Store.ImportProspects(body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) importTool(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.Header.Get("Content-Type"), "json") {
		var body struct {
			CSV string `json:"csv"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		out, err := s.Store.ImportProspects(strings.NewReader(body.CSV))
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	s.importCSV(w, r)
}

func (s *Server) ficheTool(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	out, err := s.Store.Fiche(body.ID)
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) waitTool(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID  string `json:"id"`
		Due string `json:"due"`
		Why string `json:"why"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	p, err := s.Store.WaitOnThem(body.ID, body.Due, body.Why)
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) relanceCompleteTool(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID, Due, Why, Channel string
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	p, err := s.Store.CompleteRelance(body.ID, body.Due, body.Why, body.Channel)
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) relancePlanTool(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID, Due, Why, Channel string
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	p, err := s.Store.PlanRelance(body.ID, body.Due, body.Why, body.Channel)
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) updatePersonTool(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID, Name, Org, Pole, Lead, Phone, Email string
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	p, err := s.Store.UpdatePerson(body.ID, store.Person{
		Name: body.Name, Org: body.Org, Pole: body.Pole, Lead: body.Lead,
		Phone: body.Phone, Email: body.Email,
	})
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) publicConfig(w http.ResponseWriter, r *http.Request) {
	if s.Setup == nil {
		writeJSON(w, http.StatusOK, map[string]any{"done": false})
		return
	}
	p := s.Setup.Public()
	writeJSON(w, http.StatusOK, map[string]any{
		"done": p.Done, "listen": p.Listen, "domain": p.Domain, "https": p.HTTPS,
		"user": p.User, "token_shown": p.TokenShown,
	})
}

package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"brain.op3.ch/sun221/k-crm/internal/mailacct"
)

func (s *Server) mailAccounts(w http.ResponseWriter, r *http.Request) {
	if s.Mail == nil {
		writeJSON(w, http.StatusOK, map[string]any{"accounts": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": s.Mail.List()})
}

func (s *Server) mailCreate(w http.ResponseWriter, r *http.Request) {
	if s.Mail == nil {
		writeErr(w, http.StatusInternalServerError, "mail")
		return
	}
	in, ok := decodeMail(w, r)
	if !ok {
		return
	}
	a, err := s.Mail.Create(in)
	if err != nil {
		writeErr(w, statusMail(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"account": a})
}

func (s *Server) mailUpdate(w http.ResponseWriter, r *http.Request) {
	if s.Mail == nil {
		writeErr(w, http.StatusInternalServerError, "mail")
		return
	}
	in, ok := decodeMail(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		id = in.ID
	}
	a, err := s.Mail.Update(id, in)
	if err != nil {
		writeErr(w, statusMail(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"account": a})
}

func (s *Server) mailDelete(w http.ResponseWriter, r *http.Request) {
	if s.Mail == nil {
		writeErr(w, http.StatusInternalServerError, "mail")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		in, ok := decodeMail(w, r)
		if !ok {
			return
		}
		id = in.ID
	}
	if err := s.Mail.Delete(id); err != nil {
		writeErr(w, statusMail(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) mailTest(w http.ResponseWriter, r *http.Request) {
	if s.Mail == nil {
		writeErr(w, http.StatusInternalServerError, "mail")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		in, ok := decodeMail(w, r)
		if !ok {
			return
		}
		id = in.ID
	}
	a, err := s.Mail.Test(id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "account": a, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "account": a})
}

func decodeMail(w http.ResponseWriter, r *http.Request) (mailacct.Input, bool) {
	var in mailacct.Input
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json")
			return in, false
		}
	}
	return in, true
}

func statusMail(err error) int {
	switch {
	case errors.Is(err, mailacct.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, mailacct.ErrSMTPExists):
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}

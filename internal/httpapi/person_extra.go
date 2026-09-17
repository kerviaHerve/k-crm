package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
)

func (s *Server) reactivate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Due, Why, Channel string
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	p, err := s.Store.Reactivate(r.PathValue("id"), body.Due, body.Why, body.Channel)
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) reactivateTool(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID, Due, Why, Channel string
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	p, err := s.Store.Reactivate(body.ID, body.Due, body.Why, body.Channel)
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) dropRelance(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RelanceID         string `json:"relance_id"`
		Due, Why, Channel string
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	p, err := s.Store.DeleteRelance(r.PathValue("id"), body.RelanceID, body.Due, body.Why, body.Channel)
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) dropRelanceTool(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID, RelanceID, Due, Why, Channel string
		RID                              string `json:"relance_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	rid := body.RelanceID
	if rid == "" {
		rid = body.RID
	}
	p, err := s.Store.DeleteRelance(body.ID, rid, body.Due, body.Why, body.Channel)
	if err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) personAvatarGet(w http.ResponseWriter, r *http.Request) {
	raw, ctype, err := s.Store.ReadPersonAvatar(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(raw)
}

func (s *Server) personAvatarUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid upload")
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "file required")
		return
	}
	defer f.Close()
	if hdr.Size > 800*1024 {
		writeErr(w, http.StatusBadRequest, "image too large")
		return
	}
	raw, err := io.ReadAll(io.LimitReader(f, 800*1024+1))
	if err != nil || len(raw) == 0 || len(raw) > 800*1024 {
		writeErr(w, http.StatusBadRequest, "unreadable image")
		return
	}
	if err := s.Store.SetPersonAvatar(r.PathValue("id"), raw); err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "has_avatar": true})
}

func (s *Server) personAvatarClear(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.ClearPersonAvatar(r.PathValue("id")); err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "has_avatar": false})
}

func (s *Server) personAvatarTool(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID    string `json:"id"`
		Path  string `json:"path"`
		Clear string `json:"clear"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	clear := strings.EqualFold(strings.TrimSpace(body.Clear), "true") || strings.TrimSpace(body.Clear) == "1"
	if clear {
		if err := s.Store.ClearPersonAvatar(body.ID); err != nil {
			storeHTTP(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "has_avatar": false})
		return
	}
	path := strings.TrimSpace(body.Path)
	if path == "" {
		writeErr(w, http.StatusBadRequest, "path or clear required")
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "unreadable image")
		return
	}
	if err := s.Store.SetPersonAvatar(body.ID, raw); err != nil {
		storeHTTP(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "has_avatar": true})
}

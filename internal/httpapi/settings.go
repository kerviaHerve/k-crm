package httpapi

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/setup"
	"brain.op3.ch/sun221/k-crm/internal/store"
	"brain.op3.ch/sun221/k-crm/internal/version"
)

func (s *Server) bearerOK(got string) bool {
	got = strings.TrimSpace(strings.TrimPrefix(got, "Bearer "))
	if got == "" {
		return false
	}
	if s.Keys != nil && s.Keys.Check(got) {
		return true
	}
	if s.Token != "" && subtle.ConstantTimeCompare([]byte(got), []byte(s.Token)) == 1 {
		return true
	}
	return false
}

func (s *Server) settingsMe(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{"user": "", "totp": false, "has_avatar": false, "keys": []any{}, "listen": "", "domain": "", "https": false, "activities": []string{}}
	if s.Setup != nil {
		p := s.Setup.Public()
		out["user"] = p.User
		out["totp"] = p.Done
		out["listen"] = p.Listen
		out["domain"] = p.Domain
		out["https"] = p.HTTPS
		acts := p.Activities
		if acts == nil {
			acts = []string{}
		}
		out["activities"] = acts
	}
	if s.Keys != nil {
		out["keys"] = s.Keys.List()
	}
	if s.Mail != nil {
		out["mail_accounts"] = s.Mail.List()
	} else {
		out["mail_accounts"] = []any{}
	}
	if s.DataDir != "" {
		_, err := os.Stat(filepath.Join(s.DataDir, "avatar"))
		out["has_avatar"] = err == nil
	}
	if s.Store != nil {
		if list, err := s.Store.ListBackups(); err == nil {
			out["backups"] = list
		}
	}
	out["systemd"] = os.Getenv("KCRM_SYSTEMD") == "1"
	out["version"] = version.Number
	out["revision"] = version.ShortRev()
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	if s.Setup == nil {
		writeErr(w, http.StatusUnauthorized, "wizard required")
		return
	}
	var body struct {
		Current, Next, Code string
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.Setup.ChangePassword(body.Current, body.Next, body.Code, s.now()); err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) settingsActivities(w http.ResponseWriter, r *http.Request) {
	if s.Setup == nil {
		writeErr(w, http.StatusUnauthorized, "wizard required")
		return
	}
	var body struct {
		Activities []string `json:"activities"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.Setup.SetActivities(body.Activities); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	acts := s.Setup.Public().Activities
	if acts == nil {
		acts = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"activities": acts})
}

func (s *Server) totpStart(w http.ResponseWriter, r *http.Request) {
	if s.Setup == nil {
		writeErr(w, http.StatusUnauthorized, "wizard required")
		return
	}
	var body struct {
		Password, Code string
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !s.Setup.Verify(s.Setup.User(), body.Password, body.Code, s.now()) {
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	sec, err := setup.NewTOTPSecret()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "totp")
		return
	}
	otpauth, qr, err := setup.TOTPQR(s.Setup.User(), sec)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "totp qr")
		return
	}
	s.pendingTOTP = sec
	writeJSON(w, http.StatusOK, map[string]string{
		"secret":  sec,
		"otpauth": otpauth,
		"qr":      qr,
	})
}

func (s *Server) totpConfirm(w http.ResponseWriter, r *http.Request) {
	if s.Setup == nil || s.pendingTOTP == "" {
		writeErr(w, http.StatusBadRequest, "start 2fa first")
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !setup.VerifyTOTP(s.pendingTOTP, body.Code, s.now()) {
		writeErr(w, http.StatusUnauthorized, "invalid totp")
		return
	}
	if err := s.Setup.ReplaceTOTP(s.pendingTOTP); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.pendingTOTP = ""
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) avatarGet(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(s.DataDir, "avatar")
	b, err := os.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ctype := "image/jpeg"
	if t, err := os.ReadFile(path + ".type"); err == nil {
		ctype = string(bytes.TrimSpace(t))
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b)
}

func (s *Server) avatarUpload(w http.ResponseWriter, r *http.Request) {
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
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "jpeg" && format != "png") {
		writeErr(w, http.StatusBadRequest, "jpeg or png only")
		return
	}
	if cfg.Width > 2048 || cfg.Height > 2048 {
		writeErr(w, http.StatusBadRequest, "image too large")
		return
	}
	if err := os.MkdirAll(s.DataDir, 0o700); err != nil {
		writeErr(w, http.StatusInternalServerError, "store")
		return
	}
	path := filepath.Join(s.DataDir, "avatar")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		writeErr(w, http.StatusInternalServerError, "store")
		return
	}
	ctype := "image/jpeg"
	if format == "png" {
		ctype = "image/png"
	}
	_ = os.WriteFile(path+".type", []byte(ctype), 0o600)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "has_avatar": true})
}

func (s *Server) keysList(w http.ResponseWriter, r *http.Request) {
	if s.Keys == nil {
		writeJSON(w, http.StatusOK, map[string]any{"keys": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": s.Keys.List()})
}

func (s *Server) keysCreate(w http.ResponseWriter, r *http.Request) {
	if s.Keys == nil {
		writeErr(w, http.StatusInternalServerError, "keys")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	k, plain, err := s.Keys.Create(body.Name)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"key": k, "token": plain})
}

func (s *Server) keysRevoke(w http.ResponseWriter, r *http.Request) {
	if s.Keys == nil {
		writeErr(w, http.StatusInternalServerError, "keys")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		var body struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		id = body.ID
	}
	if err := s.Keys.Revoke(id); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) keysCreateTool(w http.ResponseWriter, r *http.Request) {
	s.keysCreate(w, r)
}

func (s *Server) keysRevokeTool(w http.ResponseWriter, r *http.Request) {
	s.keysRevoke(w, r)
}

func (s *Server) backupsList(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListBackups()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": list})
}

func (s *Server) backupsCreate(w http.ResponseWriter, r *http.Request) {
	info, err := s.Store.Backup()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

func (s *Server) backupsGet(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.PathValue("name"))
	if !store.ValidBackupName(name) {
		writeErr(w, http.StatusBadRequest, "invalid backup name")
		return
	}
	path := filepath.Join(s.DataDir, "backups", name)
	if _, err := os.Stat(path); err != nil {
		writeErr(w, http.StatusNotFound, "backup not found")
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeFile(w, r, path)
}

func (s *Server) backupsRestore(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.PathValue("name"))
	if err := s.Store.StageRestore(name); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	restart := os.Getenv("KCRM_SYSTEMD") == "1"
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "restart": restart})
	if restart {
		go func() {
			time.Sleep(400 * time.Millisecond)
			os.Exit(0)
		}()
	}
}

func (s *Server) listLost(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListLost()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "store")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"people": list})
}

package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/sessions"
	"brain.op3.ch/sun221/k-crm/internal/setup"
	"brain.op3.ch/sun221/k-crm/internal/webui"
)

func (s *Server) Handler() http.Handler {
	mux := s.Routes()
	return s.gate(mux)
}

func (s *Server) gate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Setup == nil {
			next.ServeHTTP(w, r)
			return
		}
		path := r.URL.Path
		if path == "/healthz" || strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if isPublicAsset(path) || path == "/install" || path == "/install/start" || path == "/install/confirm" || path == "/install/status" || path == "/install/listen" || path == "/login" || path == "/logout" {
			next.ServeHTTP(w, r)
			return
		}
		if !s.Setup.Done() {
			if strings.HasPrefix(path, "/ui/api/") {
				writeErr(w, http.StatusUnauthorized, "wizard required")
				return
			}
			http.Redirect(w, r, "/install", http.StatusSeeOther)
			return
		}
		if s.sessionOK(r) {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(path, "/ui/api/") {
			writeErr(w, http.StatusUnauthorized, "login required")
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

func isPublicAsset(path string) bool {
	return path == "/pupitre.css" || path == "/kervia.css" || path == "/fonts.css" || path == "/app.css" || path == "/install.js" || path == "/login.js" ||
		strings.HasPrefix(path, "/fonts/") || strings.HasPrefix(path, "/brand/")
}

func (s *Server) sess() *sessions.Store {
	if s.Sessions == nil {
		s.Sessions = sessions.Memory()
	}
	return s.Sessions
}

func (s *Server) sessionOK(r *http.Request) bool {
	c, err := r.Cookie("kcrm")
	if err != nil || c.Value == "" {
		return false
	}
	return s.sess().Valid(c.Value)
}

func (s *Server) putSession(w http.ResponseWriter) {
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	id := hex.EncodeToString(raw)
	_ = s.sess().Put(id, 24*time.Hour)
	http.SetCookie(w, &http.Cookie{Name: "kcrm", Value: id, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 86400})
}

func (s *Server) installPage(w http.ResponseWriter, r *http.Request) {
	if s.Setup != nil && s.Setup.Done() {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.ServeFileFS(w, r, webui.Static(), "install.html")
}

func (s *Server) installStatus(w http.ResponseWriter, _ *http.Request) {
	out := map[string]any{"done": false, "listen": s.Listen}
	if s.Setup != nil {
		p := s.Setup.Public()
		out["done"] = p.Done
		if s.Listen == "" {
			out["listen"] = p.Listen
		}
		if p.Done && p.User != "" {
			out["user"] = p.User
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) installListen(w http.ResponseWriter, r *http.Request) {
	if s.Setup != nil && s.Setup.Done() {
		writeErr(w, http.StatusConflict, "already installed")
		return
	}
	var body struct {
		Listen string `json:"listen"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := setup.ValidateListen(body.Listen); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"listen": strings.TrimSpace(body.Listen)})
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	if s.Setup == nil || !s.Setup.Done() {
		http.Redirect(w, r, "/install", http.StatusSeeOther)
		return
	}
	http.ServeFileFS(w, r, webui.Static(), "login.html")
}

func (s *Server) installStart(w http.ResponseWriter, r *http.Request) {
	if s.Setup == nil || s.Setup.Done() {
		writeErr(w, http.StatusConflict, "already installed")
		return
	}
	var body struct {
		Listen, Domain, User, Password string
		HTTPS                          bool
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := setup.ValidateListen(body.Listen); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(body.User) == "" {
		writeErr(w, http.StatusBadRequest, "user required")
		return
	}
	hash, err := setup.HashPassword(body.Password)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	sec, err := setup.NewTOTPSecret()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "totp")
		return
	}
	s.pending = &setup.Pending{
		Listen: body.Listen, Domain: body.Domain, HTTPS: body.HTTPS,
		User: strings.TrimSpace(body.User), PassHash: hash, TOTP: sec,
	}
	otpauth, qr, err := setup.TOTPQR(s.pending.User, sec)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "totp qr")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"otpauth": otpauth,
		"secret":  sec,
		"qr":      qr,
	})
}

func (s *Server) installConfirm(w http.ResponseWriter, r *http.Request) {
	if s.Setup == nil || s.pending == nil {
		writeErr(w, http.StatusBadRequest, "start wizard first")
		return
	}
	var body struct{ Code string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !setup.VerifyTOTP(s.pending.TOTP, body.Code, s.now()) {
		writeErr(w, http.StatusUnauthorized, "invalid totp")
		return
	}
	if err := s.Setup.Commit(*s.pending); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.pending = nil
	out := map[string]any{"ok": true, "user": s.Setup.User(), "listen": s.Setup.Public().Listen}
	if s.Listen != "" && s.Listen != s.Setup.Public().Listen {
		out["restart"] = true
	}
	if !s.Setup.TokenAlreadyShown() {
		out["token"] = s.Token
		_ = s.Setup.MarkTokenShown()
	}
	s.putSession(w)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.Setup == nil || !s.Setup.Done() {
		writeErr(w, http.StatusUnauthorized, "wizard required")
		return
	}
	var body struct{ User, Password, Code string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !s.Setup.Verify(body.User, body.Password, body.Code, s.now()) {
		writeErr(w, http.StatusUnauthorized, "invalid login")
		return
	}
	s.putSession(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("kcrm"); err == nil {
		_ = s.sess().Delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "kcrm", Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

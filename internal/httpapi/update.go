package httpapi

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/update"
	"brain.op3.ch/sun221/k-crm/internal/version"
)

func (s *Server) updateEnv() *update.Env {
	if s.Update != nil {
		if s.Update.Backup == nil && s.Store != nil {
			s.Update.Backup = func() error {
				_, err := s.Store.Backup()
				return err
			}
		}
		return s.Update
	}
	e := update.Discover(s.DataDir)
	if s.Store != nil {
		e.Backup = func() error {
			_, err := s.Store.Backup()
			return err
		}
	}
	return e
}

func (s *Server) updateGet(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	st := s.updateEnv().Check(ctx, false)
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) updateCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	st := s.updateEnv().Check(ctx, true)
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) updateApply(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Minute)
	defer cancel()
	res, err := s.updateEnv().Apply(ctx)
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, update.ErrBusy) {
			code = http.StatusConflict
		}
		writeErr(w, code, err.Error())
		return
	}
	out := map[string]any{
		"ok":      true,
		"head":    res.Head,
		"bin":     res.Bin,
		"restart": os.Getenv("KCRM_SYSTEMD") == "1",
		"version": version.Number,
	}
	writeJSON(w, http.StatusOK, out)
	if os.Getenv("KCRM_SYSTEMD") == "1" {
		go func() {
			time.Sleep(400 * time.Millisecond)
			os.Exit(0)
		}()
	}
}

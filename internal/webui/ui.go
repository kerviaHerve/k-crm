package webui

import (
	"embed"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/store"
)

//go:embed templates/*.html fonts/*.woff2 assets/*.svg
var files embed.FS

var pages = template.Must(template.ParseFS(files, "templates/*.html"))

type Handler struct {
	Store *store.Store
	Now   func() time.Time
}

func Mount(mux *http.ServeMux, st *store.Store, now func() time.Time) {
	h := &Handler{Store: st, Now: now}
	mux.HandleFunc("GET /", h.home)
	mux.HandleFunc("GET /p/{id}", h.fiche)
	mux.HandleFunc("POST /ui/prospects", h.create)
	mux.HandleFunc("POST /ui/p/{id}/notes", h.note)
	mux.HandleFunc("POST /ui/p/{id}/validate", h.validate)
	mux.HandleFunc("GET /ui/fonts/SpaceGrotesk-Variable.woff2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "font/woff2")
		b, err := files.ReadFile("fonts/SpaceGrotesk-Variable.woff2")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(b)
	})
	mux.HandleFunc("GET /ui/assets/kervia-logo-ia-color-on-dark.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		b, err := files.ReadFile("assets/kervia-logo-ia-color-on-dark.svg")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(b)
	})
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	out, err := h.Store.AujourdHui(h.now())
	if err != nil {
		http.Error(w, "store", http.StatusInternalServerError)
		return
	}
	h.render(w, "home", map[string]any{
		"Day":        out,
		"Err":        r.URL.Query().Get("err"),
		"DueDefault": h.now().UTC().Add(24 * time.Hour).Format("2006-01-02"),
	})
}

func (h *Handler) fiche(w http.ResponseWriter, r *http.Request) {
	out, err := h.Store.Fiche(r.PathValue("id"))
	if err != nil {
		http.Error(w, "introuvable", http.StatusNotFound)
		return
	}
	h.render(w, "fiche", map[string]any{
		"Fiche": out,
		"Err":   r.URL.Query().Get("err"),
	})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?err=formulaire", http.StatusSeeOther)
		return
	}
	p, err := h.Store.CreateProspect(store.Person{
		Name:  r.FormValue("name"),
		Org:   r.FormValue("org"),
		Pole:  r.FormValue("pole"),
		Lead:  r.FormValue("lead"),
		Phone: r.FormValue("phone"),
		Email: r.FormValue("email"),
	}, r.FormValue("due"), r.FormValue("why"), r.FormValue("channel"))
	if err != nil {
		http.Redirect(w, r, "/?err="+errQuery(err), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/p/"+p.ID, http.StatusSeeOther)
}

func (h *Handler) note(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/p/"+id+"?err=formulaire", http.StatusSeeOther)
		return
	}
	if _, err := h.Store.AddNote(id, r.FormValue("title"), r.FormValue("body")); err != nil {
		http.Redirect(w, r, "/p/"+id+"?err="+errQuery(err), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/p/"+id, http.StatusSeeOther)
}

func (h *Handler) validate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.Store.ValidateLead(id); err != nil {
		http.Redirect(w, r, "/p/"+id+"?err="+errQuery(err), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/p/"+id, http.StatusSeeOther)
}

func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pages.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template", http.StatusInternalServerError)
	}
}

func errQuery(err error) string {
	s := err.Error()
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 120 {
		s = s[:120]
	}
	return url.QueryEscape(s)
}

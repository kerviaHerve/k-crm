package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

func icsEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func (s *Server) relanceICS(w http.ResponseWriter, r *http.Request) {
	p, err := s.Store.OpenRelance(r.PathValue("id"))
	if err != nil {
		storeHTTP(w, err)
		return
	}
	if p.Due == "" {
		writeErr(w, http.StatusNotFound, "no open relance")
		return
	}
	if _, err := time.Parse("2006-01-02", p.Due); err != nil {
		writeErr(w, http.StatusBadRequest, "bad due")
		return
	}
	stamp := s.now().UTC().Format("20060102T150405Z")
	summary := icsEscape("Relance K-CRM: " + p.Name)
	desc := icsEscape(p.Why)
	uid := icsEscape("k-crm-" + p.ID + "-" + p.Due)
	body := "BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"PRODID:-//K-CRM//relance//FR\r\n" +
		"CALSCALE:GREGORIAN\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:" + uid + "\r\n" +
		"DTSTAMP:" + stamp + "\r\n" +
		"DTSTART;VALUE=DATE:" + strings.ReplaceAll(p.Due, "-", "") + "\r\n" +
		"SUMMARY:" + summary + "\r\n" +
		"DESCRIPTION:" + desc + "\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="relance-%s.ics"`, strings.ReplaceAll(p.ID, "/", "")))
	_, _ = w.Write([]byte(body))
}

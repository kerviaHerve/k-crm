package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/mailparse"
)

func (s *Server) ingest(w http.ResponseWriter, r *http.Request) {
	raw, err := readIngestRaw(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := s.Store.IngestRaw(raw)
	if err != nil {
		if errors.Is(err, mailparse.ErrNotMessage) {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func readIngestRaw(r *http.Request) (string, error) {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/") {
		if err := r.ParseMultipartForm(maxRawIngest); err != nil {
			return "", err
		}
		if f, _, err := r.FormFile("file"); err == nil {
			defer f.Close()
			b, err := io.ReadAll(io.LimitReader(f, maxRawIngest))
			if err != nil {
				return "", err
			}
			if len(b) == 0 {
				return "", fmt.Errorf("empty file")
			}
			return string(b), nil
		}
		if v := r.FormValue("raw"); v != "" {
			return v, nil
		}
		return "", fmt.Errorf("missing file")
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, maxRawIngest))
	if err != nil {
		return "", err
	}
	if strings.Contains(ct, "json") {
		var body struct {
			Raw, Eml, Message, From, Subject, Body, Date, MessageID string
		}
		if err := json.Unmarshal(b, &body); err != nil {
			return "", err
		}
		raw := firstNonEmpty(body.Raw, body.Eml, body.Message)
		if raw != "" {
			return raw, nil
		}
		if strings.TrimSpace(body.From) == "" || strings.TrimSpace(body.Body) == "" {
			return "", fmt.Errorf("raw message required")
		}
		return synthesizeRFC822(body.From, body.Subject, body.Body, body.Date, body.MessageID), nil
	}
	if len(bytesTrim(b)) == 0 {
		return "", fmt.Errorf("raw message required")
	}
	return string(b), nil
}

const maxRawIngest = 1 << 20

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func synthesizeRFC822(from, subject, body, date, id string) string {
	if date == "" {
		date = time.Now().UTC().Format(time.RFC1123Z)
	}
	if id == "" {
		id = "<synthetic@k-crm>"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	fmt.Fprintf(&b, "Date: %s\r\n", date)
	fmt.Fprintf(&b, "Message-ID: %s\r\n", id)
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\r\n")
	}
	return b.String()
}

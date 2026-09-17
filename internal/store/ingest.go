package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/mailparse"
)

type IngestResult struct {
	Action      string   `json:"action"`
	Reason      string   `json:"reason,omitempty"`
	MessageID   string   `json:"message_id,omitempty"`
	From        string   `json:"from,omitempty"`
	Subject     string   `json:"subject,omitempty"`
	PersonID    string   `json:"person_id,omitempty"`
	PersonName  string   `json:"person_name,omitempty"`
	NoteID      string   `json:"note_id,omitempty"`
	Attachments []string `json:"attachments,omitempty"`
}

func (s *Store) IngestRaw(raw string) (IngestResult, error) {
	msg, err := mailparse.Parse([]byte(raw))
	if err != nil {
		return IngestResult{}, err
	}
	return s.ingest(msg)
}

func (s *Store) ingest(msg mailparse.Message) (IngestResult, error) {
	out := IngestResult{
		MessageID:   msg.MessageID,
		From:        msg.FromEmail,
		Subject:     msg.Subject,
		Attachments: msg.Attachments,
	}
	if msg.Skip != "" {
		out.Action = "skipped"
		out.Reason = msg.Skip
		return out, nil
	}
	if id, err := s.ingestedNote(msg.MessageID); err != nil {
		return IngestResult{}, err
	} else if id != "" {
		out.Action = "duplicate"
		out.Reason = "already filed"
		out.NoteID = id
		return out, nil
	}
	person, ok, err := s.matchMail(msg)
	if err != nil {
		return IngestResult{}, err
	}
	if !ok {
		out.Action = "unmatched"
		out.Reason = "unknown address"
		return out, nil
	}
	title := strings.TrimSpace(msg.Subject)
	if title == "" {
		title = "Mail"
	} else {
		title = "Mail: " + title
	}
	if len([]rune(title)) > 80 {
		title = string([]rune(title)[:80])
	}
	body := msg.Body
	if len(msg.Attachments) > 0 {
		body += "\n\nPièces: " + strings.Join(msg.Attachments, ", ")
	}
	n, err := s.AddNote(person.ID, title, body)
	if err != nil {
		return IngestResult{}, err
	}
	if _, err := s.db.Exec(
		`INSERT INTO ingest (message_id, person_id, note_id, from_email, subject, created_at) VALUES (?,?,?,?,?,?)`,
		msg.MessageID, person.ID, n.ID, msg.FromEmail, msg.Subject, time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		return IngestResult{}, fmt.Errorf("ingest record: %w", err)
	}
	out.Action = "filed"
	out.PersonID = person.ID
	out.PersonName = person.Name
	out.NoteID = n.ID
	return out, nil
}

func (s *Store) ingestedNote(messageID string) (string, error) {
	if messageID == "" {
		return "", nil
	}
	var id string
	err := s.db.QueryRow(`SELECT note_id FROM ingest WHERE message_id=?`, messageID).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) matchMail(msg mailparse.Message) (Person, bool, error) {
	for _, email := range mailparse.Candidates(msg) {
		p, ok, err := s.FindDuplicate(Person{Email: email})
		if err != nil {
			return Person{}, false, err
		}
		if ok {
			return p, true, nil
		}
	}
	return Person{}, false, nil
}

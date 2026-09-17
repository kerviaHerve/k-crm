package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/ids"
)

var ErrNotLost = errors.New("not lost")

func (s *Store) ListRelances(personID string) ([]Relance, error) {
	if _, err := s.GetPerson(personID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`
SELECT id, person_id, COALESCE(due,''), channel, why, open, created_at
FROM relances WHERE person_id=? ORDER BY created_at DESC, id DESC`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Relance{}
	for rows.Next() {
		var r Relance
		var open int
		if err := rows.Scan(&r.ID, &r.PersonID, &r.Due, &r.Channel, &r.Why, &open, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Open = open == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteRelance(personID, relanceID, due, why, channel string) (Person, error) {
	p, err := s.GetPerson(personID)
	if err != nil {
		return Person{}, err
	}
	if relanceID == "" {
		err := s.db.QueryRow(`SELECT id FROM relances WHERE person_id=? AND open=1 ORDER BY created_at DESC LIMIT 1`, personID).Scan(&relanceID)
		if errors.Is(err, sql.ErrNoRows) {
			return Person{}, ErrNotFound
		}
		if err != nil {
			return Person{}, err
		}
	}
	var owner string
	var open int
	err = s.db.QueryRow(`SELECT person_id, open FROM relances WHERE id=?`, relanceID).Scan(&owner, &open)
	if errors.Is(err, sql.ErrNoRows) {
		return Person{}, ErrNotFound
	}
	if err != nil {
		return Person{}, err
	}
	if owner != personID {
		return Person{}, ErrNotFound
	}
	due = strings.TrimSpace(due)
	why = strings.TrimSpace(why)
	needsNext := open == 1 && p.World == "prospect" && p.LeadState != "perdu"
	if needsNext {
		if err := parseDue(due); err != nil {
			return Person{}, fmt.Errorf("next relance required")
		}
		if why == "" {
			return Person{}, fmt.Errorf("relance why required")
		}
	} else if due != "" {
		if err := parseDue(due); err != nil {
			return Person{}, err
		}
	}
	channel = NormalizeChannel(channel)
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.Begin()
	if err != nil {
		return Person{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM relances WHERE id=?`, relanceID); err != nil {
		return Person{}, err
	}
	if due != "" && why != "" && (needsNext || open == 1) {
		if _, err := tx.Exec(`INSERT INTO relances (id,person_id,due,channel,why,open,created_at) VALUES (?,?,?,?,?,1,?)`,
			ids.New(), personID, due, channel, why, now); err != nil {
			return Person{}, err
		}
	}
	body := "Relance supprimée."
	if due != "" {
		body = "Relance supprimée. Suite le " + due + "."
	}
	if _, err := tx.Exec(`INSERT INTO notes (id,person_id,title,body,created_at) VALUES (?,?,?,?,?)`,
		ids.New(), personID, "Relance supprimée", body, now); err != nil {
		return Person{}, err
	}
	if err := tx.Commit(); err != nil {
		return Person{}, err
	}
	return s.GetPerson(personID)
}

func (s *Store) Reactivate(id, due, why, channel string) (Person, error) {
	p, err := s.GetPerson(id)
	if err != nil {
		return Person{}, err
	}
	if p.World != "prospect" {
		return Person{}, ErrNotProspect
	}
	if p.LeadState != "perdu" {
		return Person{}, ErrNotLost
	}
	due = strings.TrimSpace(due)
	why = strings.TrimSpace(why)
	if err := parseDue(due); err != nil {
		return Person{}, err
	}
	if why == "" {
		return Person{}, fmt.Errorf("relance why required")
	}
	channel = NormalizeChannel(channel)
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.Begin()
	if err != nil {
		return Person{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE people SET lead_state='en cours' WHERE id=?`, id); err != nil {
		return Person{}, err
	}
	if _, err := tx.Exec(`UPDATE relances SET open=0 WHERE person_id=? AND open=1`, id); err != nil {
		return Person{}, err
	}
	if _, err := tx.Exec(`INSERT INTO relances (id,person_id,due,channel,why,open,created_at) VALUES (?,?,?,?,?,1,?)`,
		ids.New(), id, due, channel, why, now); err != nil {
		return Person{}, err
	}
	if _, err := tx.Exec(`INSERT INTO notes (id,person_id,title,body,created_at) VALUES (?,?,?,?,?)`,
		ids.New(), id, "Réactivé", why+" · "+due, now); err != nil {
		return Person{}, err
	}
	if err := tx.Commit(); err != nil {
		return Person{}, err
	}
	return s.GetPerson(id)
}

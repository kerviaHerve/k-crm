package store

import (
	"database/sql"
	"fmt"
	"strings"
)

const (
	personCols = `p.id,p.name,p.org,p.pole,p.world,p.lead,p.lead_state,p.phone,p.email,
COALESCE(r.due,''), COALESCE(r.why,''), COALESCE(r.channel,''), COALESCE(p.heat,'')`
	personFrom = `people p LEFT JOIN relances r ON r.person_id=p.id AND r.open=1`
)

type Relance struct {
	ID        string `json:"id"`
	PersonID  string `json:"person_id"`
	Due       string `json:"due"`
	Channel   string `json:"channel"`
	Why       string `json:"why"`
	Open      bool   `json:"open"`
	CreatedAt string `json:"created_at"`
}

func NormalizeChannel(channel string) string {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "", "tel", "telephone", "téléphone", "phone":
		return "tel"
	case "attente":
		return "attente"
	default:
		return "autres"
	}
}

func NormalizeHeat(heat string) (string, error) {
	h := strings.ToLower(strings.TrimSpace(heat))
	h = strings.NewReplacer("è", "e", "é", "e", "ê", "e").Replace(h)
	switch h {
	case "", "froid", "tiede", "chaud":
		return h, nil
	case "cold":
		return "froid", nil
	case "warm":
		return "tiede", nil
	case "hot":
		return "chaud", nil
	default:
		return "", fmt.Errorf("heat must be froid, tiede or chaud")
	}
}

func scanPerson(sc interface{ Scan(dest ...any) error }) (Person, error) {
	var p Person
	err := sc.Scan(&p.ID, &p.Name, &p.Org, &p.Pole, &p.World, &p.Lead, &p.LeadState, &p.Phone, &p.Email, &p.Due, &p.Why, &p.Channel, &p.Heat)
	return p, err
}

func (s *Store) ensurePeopleHeat() error {
	rows, err := s.db.Query(`PRAGMA table_info(people)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == "heat" {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE people ADD COLUMN heat TEXT NOT NULL DEFAULT ''`)
	return err
}

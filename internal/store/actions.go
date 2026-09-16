package store

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"brain.op3.ch/sun221/k-crm/internal/ids"
)

var (
	ErrLost = fmt.Errorf("already lost")
)

type TimelineEvent struct {
	T     string `json:"t"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

type Card struct {
	Person
	Initials  string           `json:"initials"`
	When      string           `json:"when"`
	WhenLabel string           `json:"whenLabel"`
	Next      string           `json:"next"`
	Timeline  []TimelineEvent  `json:"timeline"`
}

type DayCount struct {
	Day string `json:"day"`
	N   int    `json:"n"`
}

type State struct {
	GeneratedAt string     `json:"generated_at"`
	People      []Card     `json:"people"`
	WeekLoad    []DayCount `json:"week_load"`
}

func parseDue(due string) error {
	due = strings.TrimSpace(due)
	if due == "" {
		return fmt.Errorf("due must be YYYY-MM-DD")
	}
	_, err := time.Parse("2006-01-02", due)
	if err != nil {
		return fmt.Errorf("due must be YYYY-MM-DD")
	}
	return nil
}

func (s *Store) PlanRelance(personID, due, why, channel string) (Person, error) {
	p, err := s.GetPerson(personID)
	if err != nil {
		return Person{}, err
	}
	if p.LeadState == "perdu" {
		return Person{}, ErrLost
	}
	due = strings.TrimSpace(due)
	why = strings.TrimSpace(why)
	if err := parseDue(due); err != nil {
		return Person{}, err
	}
	if why == "" {
		return Person{}, fmt.Errorf("relance why required")
	}
	if channel == "" {
		channel = "tel"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.Begin()
	if err != nil {
		return Person{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE relances SET open=0 WHERE person_id=? AND open=1`, personID); err != nil {
		return Person{}, err
	}
	if _, err := tx.Exec(`INSERT INTO relances (id,person_id,due,channel,why,open,created_at) VALUES (?,?,?,?,?,1,?)`,
		ids.New(), personID, due, channel, why, now); err != nil {
		return Person{}, err
	}
	if p.World == "prospect" && p.LeadState == "nouveau" {
		if _, err := tx.Exec(`UPDATE people SET lead_state='en cours' WHERE id=?`, personID); err != nil {
			return Person{}, err
		}
		p.LeadState = "en cours"
	}
	if _, err := tx.Exec(`INSERT INTO notes (id,person_id,title,body,created_at) VALUES (?,?,?,?,?)`,
		ids.New(), personID, "Relance posée", why+" · "+due, now); err != nil {
		return Person{}, err
	}
	if err := tx.Commit(); err != nil {
		return Person{}, err
	}
	p.Due, p.Why, p.Channel = due, why, channel
	return p, nil
}

func (s *Store) WaitOnThem(id, due, why string) (Person, error) {
	if strings.TrimSpace(why) == "" {
		why = "En attente d'eux"
	}
	p, err := s.PlanRelance(id, due, why, "attente")
	if err != nil {
		return Person{}, err
	}
	if p.World == "prospect" && p.LeadState != "perdu" {
		if _, err := s.db.Exec(`UPDATE people SET lead_state='en attente' WHERE id=?`, id); err != nil {
			return Person{}, err
		}
		p.LeadState = "en attente"
	}
	p.Channel = "attente"
	return p, nil
}

func (s *Store) CompleteRelance(personID, due, why, channel string) (Person, error) {
	p, err := s.GetPerson(personID)
	if err != nil {
		return Person{}, err
	}
	if p.LeadState == "perdu" {
		return Person{}, ErrLost
	}
	due = strings.TrimSpace(due)
	why = strings.TrimSpace(why)
	if p.World == "prospect" {
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
	if channel == "" {
		channel = "tel"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.Begin()
	if err != nil {
		return Person{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE relances SET open=0 WHERE person_id=? AND open=1`, personID); err != nil {
		return Person{}, err
	}
	if due != "" {
		if why == "" {
			why = "suite"
		}
		if _, err := tx.Exec(`INSERT INTO relances (id,person_id,due,channel,why,open,created_at) VALUES (?,?,?,?,?,1,?)`,
			ids.New(), personID, due, channel, why, now); err != nil {
			return Person{}, err
		}
	}
	body := "Relance faite."
	if due != "" {
		body = "Relance faite. Suite le " + due + "."
	}
	if _, err := tx.Exec(`INSERT INTO notes (id,person_id,title,body,created_at) VALUES (?,?,?,?,?)`,
		ids.New(), personID, "Relance faite", body, now); err != nil {
		return Person{}, err
	}
	if err := tx.Commit(); err != nil {
		return Person{}, err
	}
	p.Due, p.Why, p.Channel = due, why, channel
	return p, nil
}

func (s *Store) MarkLost(id, why string) (Person, error) {
	p, err := s.GetPerson(id)
	if err != nil {
		return Person{}, err
	}
	if p.World != "prospect" {
		return Person{}, ErrNotProspect
	}
	why = strings.TrimSpace(why)
	if why == "" {
		why = "Marqué perdu."
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.Begin()
	if err != nil {
		return Person{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE people SET lead_state='perdu' WHERE id=?`, id); err != nil {
		return Person{}, err
	}
	if _, err := tx.Exec(`UPDATE relances SET open=0 WHERE person_id=? AND open=1`, id); err != nil {
		return Person{}, err
	}
	if _, err := tx.Exec(`INSERT INTO notes (id,person_id,title,body,created_at) VALUES (?,?,?,?,?)`,
		ids.New(), id, "Perdu", why, now); err != nil {
		return Person{}, err
	}
	if err := tx.Commit(); err != nil {
		return Person{}, err
	}
	p.LeadState = "perdu"
	p.Due, p.Why, p.Channel = "", "", ""
	return p, nil
}

func (s *Store) Search(q string) ([]Person, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return []Person{}, nil
	}
	like := "%" + q + "%"
	return s.queryPeople(`
SELECT p.id,p.name,p.org,p.pole,p.world,p.lead,p.lead_state,p.phone,p.email,
       COALESCE(r.due,''), COALESCE(r.why,''), COALESCE(r.channel,'')
FROM people p
LEFT JOIN relances r ON r.person_id=p.id AND r.open=1
WHERE p.name LIKE ? OR p.org LIKE ? OR p.pole LIKE ? OR p.lead LIKE ? OR p.phone LIKE ? OR p.email LIKE ?
ORDER BY p.name`, like, like, like, like, like, like)
}

func (s *Store) ListWorld(world string) ([]Person, error) {
	return s.queryPeople(`
SELECT p.id,p.name,p.org,p.pole,p.world,p.lead,p.lead_state,p.phone,p.email,
       COALESCE(r.due,''), COALESCE(r.why,''), COALESCE(r.channel,'')
FROM people p
LEFT JOIN relances r ON r.person_id=p.id AND r.open=1
WHERE p.world=?
ORDER BY p.name`, world)
}

func initials(name string) string {
	parts := strings.Fields(name)
	out := []rune{}
	for _, p := range parts {
		for _, r := range p {
			if unicode.IsLetter(r) {
				out = append(out, unicode.ToUpper(r))
				break
			}
		}
		if len(out) == 2 {
			break
		}
	}
	if len(out) == 0 {
		return "?"
	}
	return string(out)
}

func classify(p Person, day string, weekEnd string) (when, label, next string) {
	if p.LeadState == "perdu" {
		return "none", "Perdu", ""
	}
	if p.Due == "" {
		if p.World == "prospect" {
			return "orphan", "Sans relance", ""
		}
		return "none", "Pas de suite", ""
	}
	next = p.Why
	switch {
	case p.Due < day:
		return "overdue", "En retard · "+p.Due, next
	case p.Due == day:
		return "today", "Aujourd'hui", next
	case p.Due <= weekEnd:
		return "week", "Le "+p.Due, next
	default:
		return "week", "Le "+p.Due, next
	}
}

func (s *Store) Snapshot(now time.Time) (State, error) {
	day := now.UTC().Format("2006-01-02")
	weekEnd := now.UTC().AddDate(0, 0, 7).Format("2006-01-02")
	people, err := s.queryPeople(`
SELECT p.id,p.name,p.org,p.pole,p.world,p.lead,p.lead_state,p.phone,p.email,
       COALESCE(r.due,''), COALESCE(r.why,''), COALESCE(r.channel,'')
FROM people p
LEFT JOIN relances r ON r.person_id=p.id AND r.open=1
ORDER BY p.name`)
	if err != nil {
		return State{}, err
	}
	out := State{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		People:      []Card{},
		WeekLoad:    []DayCount{{Day: "Lun"}, {Day: "Mar"}, {Day: "Mer"}, {Day: "Jeu"}, {Day: "Ven"}, {Day: "Sam"}, {Day: "Dim"}},
	}
	monday := now.UTC()
	for monday.Weekday() != time.Monday {
		monday = monday.AddDate(0, 0, -1)
	}
	counts := map[string]int{}
	for i := 0; i < 7; i++ {
		counts[monday.AddDate(0, 0, i).Format("2006-01-02")] = 0
	}
	for _, p := range people {
		notes, err := s.Notes(p.ID)
		if err != nil {
			return State{}, err
		}
		tl := []TimelineEvent{}
		for _, n := range notes {
			t := n.CreatedAt
			if len(t) >= 10 {
				t = t[:10]
			}
			tl = append(tl, TimelineEvent{T: t, Title: n.Title, Body: n.Body})
		}
		when, label, next := classify(p, day, weekEnd)
		out.People = append(out.People, Card{
			Person:    p,
			Initials:  initials(p.Name),
			When:      when,
			WhenLabel: label,
			Next:      next,
			Timeline:  tl,
		})
		if p.Due != "" {
			if _, ok := counts[p.Due]; ok {
				counts[p.Due]++
			}
		}
	}
	for i := 0; i < 7; i++ {
		d := monday.AddDate(0, 0, i).Format("2006-01-02")
		out.WeekLoad[i].N = counts[d]
	}
	return out, nil
}

type ImportError struct {
	Line   int    `json:"line"`
	Name   string `json:"name,omitempty"`
	Reason string `json:"reason"`
}

type ImportResult struct {
	Created int           `json:"created"`
	Skipped int           `json:"skipped"`
	Errors  []ImportError `json:"errors"`
}

func (s *Store) ImportProspects(r io.Reader) (ImportResult, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	head, err := cr.Read()
	if err != nil {
		return ImportResult{}, fmt.Errorf("csv header required")
	}
	idx := map[string]int{}
	for i, h := range head {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	if _, ok := idx["name"]; !ok {
		return ImportResult{}, fmt.Errorf("csv needs a name column")
	}
	out := ImportResult{Errors: []ImportError{}}
	line := 1
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		line++
		if err != nil {
			out.Skipped++
			out.Errors = append(out.Errors, ImportError{Line: line, Reason: "bad row"})
			continue
		}
		get := func(k string) string {
			i, ok := idx[k]
			if !ok || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		p := Person{
			Name:  get("name"),
			Org:   get("org"),
			Pole:  get("pole"),
			Lead:  get("lead"),
			Phone: get("phone"),
			Email: get("email"),
		}
		due, why, channel := get("due"), get("why"), get("channel")
		if _, err := s.CreateProspect(p, due, why, channel); err != nil {
			out.Skipped++
			out.Errors = append(out.Errors, ImportError{Line: line, Name: p.Name, Reason: err.Error()})
			continue
		}
		out.Created++
		if out.Created+out.Skipped >= 2000 {
			out.Errors = append(out.Errors, ImportError{Line: line, Reason: "stopped at 2000 rows"})
			break
		}
	}
	return out, nil
}

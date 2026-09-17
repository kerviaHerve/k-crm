package store

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
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
	Initials  string          `json:"initials"`
	When      string          `json:"when"`
	WhenLabel string          `json:"whenLabel"`
	Next      string          `json:"next"`
	Timeline  []TimelineEvent `json:"timeline"`
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
	hits, err := s.SearchHits(q)
	if err != nil {
		return nil, err
	}
	out := make([]Person, 0, len(hits))
	seen := map[string]bool{}
	for _, h := range hits {
		if seen[h.ID] {
			continue
		}
		seen[h.ID] = true
		out = append(out, h.Person)
	}
	return out, nil
}

type SearchHit struct {
	Person
	Score   int    `json:"score"`
	Match   string `json:"match"`
	Snippet string `json:"snippet,omitempty"`
}

func fold(s string) string {
	r := strings.NewReplacer(
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"à", "a", "â", "a", "ä", "a",
		"î", "i", "ï", "i",
		"ô", "o", "ö", "o",
		"ù", "u", "û", "u", "ü", "u",
		"ç", "c", "œ", "oe", "æ", "ae",
		"É", "e", "È", "e", "Ê", "e", "Ë", "e",
		"À", "a", "Â", "a", "Ä", "a",
		"Î", "i", "Ï", "i",
		"Ô", "o", "Ö", "o",
		"Ù", "u", "Û", "u", "Ü", "u",
		"Ç", "c",
	)
	return strings.ToLower(r.Replace(s))
}

func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (s *Store) FindDuplicate(p Person) (Person, bool, error) {
	all, err := s.queryPeople(`
SELECT id,name,org,pole,world,lead,lead_state,phone,email,'','',''
FROM people`)
	if err != nil {
		return Person{}, false, err
	}
	email := strings.ToLower(strings.TrimSpace(p.Email))
	phone := digits(p.Phone)
	nameKey := fold(strings.TrimSpace(p.Name)) + "\n" + fold(strings.TrimSpace(p.Org))
	for _, cur := range all {
		if email != "" && strings.ToLower(strings.TrimSpace(cur.Email)) == email {
			return cur, true, nil
		}
		if len(phone) >= 8 && digits(cur.Phone) == phone {
			return cur, true, nil
		}
		if fold(strings.TrimSpace(cur.Name))+"\n"+fold(strings.TrimSpace(cur.Org)) == nameKey {
			return cur, true, nil
		}
	}
	return Person{}, false, nil
}

func (s *Store) ListLost() ([]Person, error) {
	return s.queryPeople(`
SELECT p.id,p.name,p.org,p.pole,p.world,p.lead,p.lead_state,p.phone,p.email,'','',''
FROM people p WHERE p.lead_state='perdu' ORDER BY p.name`)
}

func (s *Store) OpenRelance(id string) (Person, error) {
	list, err := s.queryPeople(`
SELECT p.id,p.name,p.org,p.pole,p.world,p.lead,p.lead_state,p.phone,p.email,
       COALESCE(r.due,''), COALESCE(r.why,''), COALESCE(r.channel,'')
FROM people p
LEFT JOIN relances r ON r.person_id=p.id AND r.open=1
WHERE p.id=?
ORDER BY r.due DESC
LIMIT 1`, id)
	if err != nil {
		return Person{}, err
	}
	if len(list) == 0 {
		return Person{}, ErrNotFound
	}
	return list[0], nil
}

func snippet(text, token string) string {
	low := fold(text)
	i := strings.Index(low, token)
	if i < 0 {
		if len(text) > 80 {
			return text[:80] + "…"
		}
		return text
	}
	start := i - 24
	if start < 0 {
		start = 0
	}
	end := i + len(token) + 32
	runes := []rune(text)
	if end > len(runes) {
		end = len(runes)
	}
	if start > len(runes) {
		start = 0
	}
	out := string(runes[start:end])
	if start > 0 {
		out = "…" + out
	}
	if end < len(runes) {
		out += "…"
	}
	return out
}

func (s *Store) SearchHits(q string) ([]SearchHit, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return []SearchHit{}, nil
	}
	tokens := strings.Fields(fold(q))
	if len(tokens) == 0 {
		return []SearchHit{}, nil
	}
	people, err := s.queryPeople(`
SELECT p.id,p.name,p.org,p.pole,p.world,p.lead,p.lead_state,p.phone,p.email,
       COALESCE(r.due,''), COALESCE(r.why,''), COALESCE(r.channel,'')
FROM people p
LEFT JOIN relances r ON r.person_id=p.id AND r.open=1
ORDER BY p.name`)
	if err != nil {
		return nil, err
	}
	notesBy := map[string][]Note{}
	rows, err := s.db.Query(`SELECT id,person_id,title,body,created_at FROM notes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.PersonID, &n.Title, &n.Body, &n.CreatedAt); err != nil {
			return nil, err
		}
		notesBy[n.PersonID] = append(notesBy[n.PersonID], n)
	}
	var hits []SearchHit
	for _, p := range people {
		best := SearchHit{Person: p, Score: -1}
		hay := map[string]string{
			"nom":     p.Name,
			"org":     p.Org,
			"pole":    p.Pole,
			"lead":    p.Lead,
			"email":   p.Email,
			"phone":   p.Phone,
			"relance": p.Why,
			"etat":    p.LeadState + " " + p.World,
		}
		ok := true
		for _, tok := range tokens {
			matched := false
			for field, val := range hay {
				fv := fold(val)
				if fv == "" {
					continue
				}
				score := 0
				snip := ""
				switch {
				case fv == tok && field == "nom":
					score = 100
				case strings.HasPrefix(fv, tok) && field == "nom":
					score = 80
				case strings.Contains(fv, tok) && field == "nom":
					score = 60
					snip = snippet(val, tok)
				case field == "phone" && strings.Contains(digits(val), digits(tok)) && digits(tok) != "":
					score = 50
					snip = val
				case strings.Contains(fv, tok):
					score = 40
					if field == "org" || field == "lead" {
						score = 45
					}
					snip = snippet(val, tok)
				}
				if score > 0 {
					matched = true
					if score > best.Score {
						best.Score = score
						best.Match = field
						best.Snippet = snip
					}
				}
			}
			for _, n := range notesBy[p.ID] {
				blob := fold(n.Title + " " + n.Body)
				if strings.Contains(blob, tok) {
					matched = true
					score := 22
					if strings.Contains(fold(n.Title), tok) {
						score = 28
					}
					if score > best.Score {
						best.Score = score
						best.Match = "note"
						best.Snippet = snippet(n.Title+": "+n.Body, tok)
					}
				}
			}
			if !matched {
				ok = false
				break
			}
		}
		if ok && best.Score >= 0 {
			hits = append(hits, best)
		}
	}
	sortHits(hits)
	if len(hits) > 50 {
		hits = hits[:50]
	}
	return hits, nil
}

func sortHits(hits []SearchHit) {
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Name < hits[j].Name
	})
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
		return "overdue", "En retard · " + p.Due, next
	case p.Due == day:
		return "today", "Aujourd'hui", next
	case p.Due <= weekEnd:
		return "week", "Le " + p.Due, next
	default:
		return "week", "Le " + p.Due, next
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

func (s *Store) WriteCSV(w io.Writer, now time.Time) error {
	st, err := s.Snapshot(now)
	if err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"id", "name", "org", "pole", "world", "lead_state", "lead", "phone", "email", "due", "why", "when"}); err != nil {
		return err
	}
	for _, p := range st.People {
		if err := cw.Write([]string{p.ID, p.Name, p.Org, p.Pole, p.World, p.LeadState, p.Lead, p.Phone, p.Email, p.Due, p.Why, p.When}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
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

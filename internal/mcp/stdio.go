package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/catalog"
	"brain.op3.ch/sun221/k-crm/internal/store"
)

type Server struct {
	Store *store.Store
	Now   func() time.Time
}

type rpc struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Server) Serve(in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)
	for {
		msg, err := readMsg(r)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		var req rpc
		if err := json.Unmarshal(msg, &req); err != nil {
			continue
		}
		if req.Method == "notifications/initialized" || strings.HasPrefix(req.Method, "notifications/") {
			continue
		}
		resp := s.handle(req)
		if req.ID == nil {
			continue
		}
		if err := writeMsg(out, resp); err != nil {
			return err
		}
	}
}

func (s *Server) handle(req rpc) rpc {
	out := rpc{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		out.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "k-crm", "version": "0.1.0"},
		}
	case "ping":
		out.Result = map[string]any{}
	case "tools/list":
		out.Result = map[string]any{"tools": tools()}
	case "tools/call":
		result, err := s.call(req.Params)
		if err != nil {
			out.Error = &rpcError{Code: -32000, Message: err.Error()}
		} else {
			out.Result = result
		}
	default:
		out.Error = &rpcError{Code: -32601, Message: "method not found"}
	}
	return out
}

func tools() []map[string]any {
	return catalog.MCP()
}

func (s *Server) call(params json.RawMessage) (map[string]any, error) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	switch p.Name {
	case "crm_aujourd_hui":
		out, err := s.Store.AujourdHui(s.now())
		if err != nil {
			return nil, err
		}
		return textResult(out)
	case "crm_creer_personne":
		var args struct {
			Name, Org, Pole, Lead, Phone, Email, Due, Why, Channel string
		}
		if len(p.Arguments) > 0 {
			if err := json.Unmarshal(p.Arguments, &args); err != nil {
				return nil, err
			}
		}
		person, err := s.Store.CreateProspect(store.Person{
			Name: args.Name, Org: args.Org, Pole: args.Pole, Lead: args.Lead,
			Phone: args.Phone, Email: args.Email,
		}, args.Due, args.Why, args.Channel)
		if err != nil {
			return nil, err
		}
		return textResult(person)
	case "crm_noter":
		var args struct{ ID, Title, Body string }
		if len(p.Arguments) > 0 {
			if err := json.Unmarshal(p.Arguments, &args); err != nil {
				return nil, err
			}
		}
		n, err := s.Store.AddNote(args.ID, args.Title, args.Body)
		if err != nil {
			return nil, err
		}
		return textResult(n)
	case "crm_valider_lead":
		var args struct{ ID string }
		if len(p.Arguments) > 0 {
			if err := json.Unmarshal(p.Arguments, &args); err != nil {
				return nil, err
			}
		}
		person, err := s.Store.ValidateLead(args.ID)
		if err != nil {
			return nil, err
		}
		return textResult(person)
	case "crm_marquer_perdu":
		var args struct{ ID, Why string }
		if len(p.Arguments) > 0 {
			if err := json.Unmarshal(p.Arguments, &args); err != nil {
				return nil, err
			}
		}
		person, err := s.Store.MarkLost(args.ID, args.Why)
		if err != nil {
			return nil, err
		}
		return textResult(person)
	case "crm_relancer":
		var args struct{ ID, Due, Why, Channel string }
		if len(p.Arguments) > 0 {
			if err := json.Unmarshal(p.Arguments, &args); err != nil {
				return nil, err
			}
		}
		person, err := s.Store.CompleteRelance(args.ID, args.Due, args.Why, args.Channel)
		if err != nil {
			return nil, err
		}
		return textResult(person)
	case "crm_reporter":
		var args struct{ ID, Due, Why, Channel string }
		if len(p.Arguments) > 0 {
			if err := json.Unmarshal(p.Arguments, &args); err != nil {
				return nil, err
			}
		}
		person, err := s.Store.PlanRelance(args.ID, args.Due, args.Why, args.Channel)
		if err != nil {
			return nil, err
		}
		return textResult(person)
	case "crm_chercher":
		var args struct{ Q string }
		if len(p.Arguments) > 0 {
			if err := json.Unmarshal(p.Arguments, &args); err != nil {
				return nil, err
			}
		}
		hits, err := s.Store.Search(args.Q)
		if err != nil {
			return nil, err
		}
		return textResult(hits)
	case "crm_importer":
		var args struct {
			CSV string `json:"csv"`
		}
		if len(p.Arguments) > 0 {
			if err := json.Unmarshal(p.Arguments, &args); err != nil {
				return nil, err
			}
		}
		out, err := s.Store.ImportProspects(strings.NewReader(args.CSV))
		if err != nil {
			return nil, err
		}
		return textResult(out)
	case "crm_fiche":
		var args struct {
			ID string `json:"id"`
		}
		if len(p.Arguments) > 0 {
			if err := json.Unmarshal(p.Arguments, &args); err != nil {
				return nil, err
			}
		}
		out, err := s.Store.Fiche(args.ID)
		if err != nil {
			return nil, err
		}
		return textResult(out)
	case "crm_attendre":
		var args struct {
			ID  string `json:"id"`
			Due string `json:"due"`
			Why string `json:"why"`
		}
		if len(p.Arguments) > 0 {
			if err := json.Unmarshal(p.Arguments, &args); err != nil {
				return nil, err
			}
		}
		person, err := s.Store.WaitOnThem(args.ID, args.Due, args.Why)
		if err != nil {
			return nil, err
		}
		return textResult(person)
	case "crm_modifier":
		var args struct {
			ID, Name, Org, Pole, Lead, Phone, Email string
		}
		if len(p.Arguments) > 0 {
			if err := json.Unmarshal(p.Arguments, &args); err != nil {
				return nil, err
			}
		}
		person, err := s.Store.UpdatePerson(args.ID, store.Person{
			Name: args.Name, Org: args.Org, Pole: args.Pole, Lead: args.Lead,
			Phone: args.Phone, Email: args.Email,
		})
		if err != nil {
			return nil, err
		}
		return textResult(person)
	case "crm_exporter":
		var buf bytes.Buffer
		if err := s.Store.WriteCSV(&buf, s.now()); err != nil {
			return nil, err
		}
		return textResult(map[string]string{"csv": buf.String()})
	case "crm_etat":
		out, err := s.Store.Snapshot(s.now())
		if err != nil {
			return nil, err
		}
		return textResult(out)
	default:
		return nil, fmt.Errorf("unknown tool")
	}
}

func textResult(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []map[string]string{{"type": "text", "text": string(b)}},
	}, nil
}

func readMsg(r *bufio.Reader) ([]byte, error) {
	n := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		low := strings.ToLower(line)
		if strings.HasPrefix(low, "content-length:") {
			n, err = strconv.Atoi(strings.TrimSpace(line[len("Content-Length:"):]))
			if err != nil {
				return nil, err
			}
		}
	}
	if n <= 0 {
		return nil, io.EOF
	}
	buf := make([]byte, n)
	_, err := io.ReadFull(r, buf)
	return buf, err
}

func writeMsg(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(b), b)
	return err
}

package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

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
	return []map[string]any{
		{
			"name":        "crm_aujourd_hui",
			"description": "Relances en retard, dues aujourd'hui, et prospects sans suite.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "crm_creer_personne",
			"description": "Cree un prospect. due (YYYY-MM-DD) et why sont obligatoires. Jamais un client.",
			"inputSchema": map[string]any{
				"type":     "object",
				"required": []string{"name", "due", "why"},
				"properties": map[string]any{
					"name":    map[string]any{"type": "string"},
					"org":     map[string]any{"type": "string"},
					"pole":    map[string]any{"type": "string"},
					"lead":    map[string]any{"type": "string"},
					"phone":   map[string]any{"type": "string"},
					"email":   map[string]any{"type": "string"},
					"due":     map[string]any{"type": "string"},
					"why":     map[string]any{"type": "string"},
					"channel": map[string]any{"type": "string"},
				},
			},
		},
	}
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

package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"brain.op3.ch/sun221/k-crm/internal/store"
)

func frame(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(b), b))
}

func TestToolsCallCreateAndAujourdHui(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	in := bytes.Join([][]byte{
		frame(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}}),
		frame(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}),
		frame(map[string]any{
			"jsonrpc": "2.0", "id": 3, "method": "tools/call",
			"params": map[string]any{
				"name": "crm_creer_personne",
				"arguments": map[string]any{
					"name": "Lea Morel", "pole": "Exonik", "due": "2026-09-15", "why": "devis",
				},
			},
		}),
		frame(map[string]any{"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": map[string]any{"name": "crm_aujourd_hui"}}),
	}, nil)
	var out bytes.Buffer
	srv := &Server{Store: st, Now: func() time.Time { return time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC) }}
	if err := srv.Serve(bytes.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	if !strings.Contains(body, "crm_creer_personne") {
		t.Fatalf("missing tool in list: %s", body)
	}
	if !strings.Contains(body, "Lea Morel") {
		t.Fatalf("missing created person: %s", body)
	}
	if !strings.Contains(body, "overdue") {
		t.Fatalf("missing aujourd'hui: %s", body)
	}
}

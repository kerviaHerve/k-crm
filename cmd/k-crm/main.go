package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"brain.op3.ch/sun221/k-crm/internal/agentkeys"
	"brain.op3.ch/sun221/k-crm/internal/httpapi"
	"brain.op3.ch/sun221/k-crm/internal/mailacct"
	"brain.op3.ch/sun221/k-crm/internal/mcp"
	"brain.op3.ch/sun221/k-crm/internal/sessions"
	"brain.op3.ch/sun221/k-crm/internal/setup"
	"brain.op3.ch/sun221/k-crm/internal/store"
)

func main() {
	log.SetFlags(0)
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	cmd := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd = args[0]
		args = args[1:]
	}
	fs := flag.NewFlagSet("k-crm", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:8740", "ip:port, loopback by default")
	dataDir := fs.String("data", "./data", "directory for sqlite and token")
	from := fs.String("from", "", "backup file name for restore")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := os.MkdirAll(*dataDir, 0o700); err != nil {
		return err
	}
	st, err := store.Open(filepath.Join(*dataDir, "k-crm.db"))
	if err != nil {
		return err
	}
	defer st.Close()
	switch cmd {
	case "mcp":
		token, err := loadOrCreateToken(filepath.Join(*dataDir, "token"))
		if err != nil {
			return err
		}
		keys, err := agentkeys.Open(*dataDir, token)
		if err != nil {
			return err
		}
		mail, err := mailacct.Open(*dataDir)
		if err != nil {
			return err
		}
		return (&mcp.Server{Store: st, Keys: keys, Mail: mail}).Serve(os.Stdin, os.Stdout)
	case "backup":
		info, err := st.Backup()
		if err != nil {
			return err
		}
		log.Printf("backup %s (%d bytes)", info.Name, info.Size)
		return nil
	case "restore":
		if strings.TrimSpace(*from) == "" {
			return fmt.Errorf("restore needs -from NAME.db")
		}
		if err := st.StageRestore(*from); err != nil {
			return err
		}
		log.Printf("restore staged; next serve applies %s", *from)
		return nil
	case "serve":
		token, err := loadOrCreateToken(filepath.Join(*dataDir, "token"))
		if err != nil {
			return err
		}
		keys, err := agentkeys.Open(*dataDir, token)
		if err != nil {
			return err
		}
		cfg, err := setup.Open(*dataDir)
		if err != nil {
			return err
		}
		sess, err := sessions.Open(*dataDir)
		if err != nil {
			return err
		}
		mail, err := mailacct.Open(*dataDir)
		if err != nil {
			return err
		}
		listenSet := false
		fs.Visit(func(f *flag.Flag) {
			if f.Name == "listen" {
				listenSet = true
			}
		})
		addr := pickListen(*listen, listenSet, cfg)
		if err := rejectWildcard(addr); err != nil {
			return err
		}
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		srv := &httpapi.Server{Store: st, Token: token, Setup: cfg, Keys: keys, Mail: mail, Sessions: sess, DataDir: *dataDir, Listen: ln.Addr().String()}
		log.Printf("k-crm listen %s", ln.Addr())
		log.Printf("token file %s", filepath.Join(*dataDir, "token"))
		if !cfg.Done() {
			log.Printf("wizard pending on /install")
		}
		return http.Serve(ln, srv.Routes())
	default:
		return fmt.Errorf("unknown command %q (serve|mcp|backup|restore)", cmd)
	}
}

func pickListen(flagValue string, flagSet bool, cfg *setup.File) string {
	if !flagSet && cfg != nil && cfg.Done() {
		if l := strings.TrimSpace(cfg.Public().Listen); l != "" {
			return l
		}
	}
	return flagValue
}

func rejectWildcard(listen string) error {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return err
	}
	if host == "0.0.0.0" || host == "*" || host == "" || host == "::" {
		return fmt.Errorf("refusing wildcard bind %q, pass an explicit IP", listen)
	}
	return nil
}

func loadOrCreateToken(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		tok := strings.TrimSpace(string(b))
		if tok == "" {
			return "", fmt.Errorf("empty token file")
		}
		return tok, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(raw)
	if err := os.WriteFile(path, []byte(tok+"\n"), 0o600); err != nil {
		return "", err
	}
	return tok, nil
}

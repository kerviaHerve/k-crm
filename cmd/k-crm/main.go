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

	"brain.op3.ch/sun221/k-crm/internal/httpapi"
	"brain.op3.ch/sun221/k-crm/internal/store"
)

func main() {
	log.SetFlags(0)
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("k-crm", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:8740", "ip:port, loopback by default")
	dataDir := fs.String("data", "./data", "directory for sqlite and token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := rejectWildcard(*listen); err != nil {
		return err
	}
	if err := os.MkdirAll(*dataDir, 0o700); err != nil {
		return err
	}
	token, err := loadOrCreateToken(filepath.Join(*dataDir, "token"))
	if err != nil {
		return err
	}
	st, err := store.Open(filepath.Join(*dataDir, "k-crm.db"))
	if err != nil {
		return err
	}
	defer st.Close()
	srv := &httpapi.Server{Store: st, Token: token}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	log.Printf("k-crm listen %s", ln.Addr())
	log.Printf("token file %s", filepath.Join(*dataDir, "token"))
	return http.Serve(ln, srv.Routes())
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

package mailacct

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestProbeIMAPLoginSelect(t *testing.T) {
	ln := listen(t)
	defer ln.Close()
	go serveIMAP(t, ln)
	host, port := split(t, ln.Addr().String())
	err := Probe(Account{
		Kind: KindIMAP, Host: host, Port: port, Security: SecNone,
		Username: "crm@net6.ch", Folder: "CRM",
	}, "s3cret")
	if err != nil {
		t.Fatal(err)
	}
}

func TestProbeIMAPBadLogin(t *testing.T) {
	ln := listen(t)
	defer ln.Close()
	go serveIMAP(t, ln)
	host, port := split(t, ln.Addr().String())
	err := Probe(Account{
		Kind: KindIMAP, Host: host, Port: port, Security: SecNone,
		Username: "crm@net6.ch", Folder: "CRM",
	}, "wrong")
	if err == nil || !strings.Contains(err.Error(), "login") {
		t.Fatalf("got %v", err)
	}
}

func TestProbeSMTPAuth(t *testing.T) {
	ln := listen(t)
	defer ln.Close()
	go serveSMTP(t, ln)
	host, port := split(t, ln.Addr().String())
	err := Probe(Account{
		Kind: KindSMTP, Host: host, Port: port, Security: SecNone,
		Username: "herve@net6.ch", From: "herve@net6.ch",
	}, "s3cret")
	if err != nil {
		t.Fatal(err)
	}
}

func TestStoreTestRecordsOK(t *testing.T) {
	ln := listen(t)
	defer ln.Close()
	go serveIMAP(t, ln)
	host, port := split(t, ln.Addr().String())
	s := openT(t)
	a, err := s.Create(Input{
		Name: "CRM", Kind: KindIMAP, Host: host, Port: Port(port),
		Security: SecNone, Username: "crm@net6.ch", Password: "s3cret", Folder: "CRM",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Test(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastOK == "" || got.LastError != "" {
		t.Fatalf("%+v", got)
	}
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

func split(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, p, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	var port int
	fmt.Sscanf(p, "%d", &port)
	return host, port
}

func serveIMAP(t *testing.T, ln net.Listener) {
	t.Helper()
	c, err := ln.Accept()
	if err != nil {
		return
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	w := bufio.NewWriter(c)
	r := bufio.NewReader(c)
	fmt.Fprint(w, "* OK kcrm test\r\n")
	w.Flush()
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			return
		}
		tag, rest := parts[0], parts[1]
		switch {
		case strings.HasPrefix(rest, "LOGIN "):
			if strings.Contains(rest, `"s3cret"`) {
				fmt.Fprintf(w, "%s OK login\r\n", tag)
			} else {
				fmt.Fprintf(w, "%s NO login\r\n", tag)
			}
		case strings.HasPrefix(rest, "SELECT "):
			fmt.Fprintf(w, "* 0 EXISTS\r\n%s OK select\r\n", tag)
		case strings.HasPrefix(rest, "LOGOUT"):
			fmt.Fprintf(w, "* BYE\r\n%s OK logout\r\n", tag)
			w.Flush()
			return
		default:
			fmt.Fprintf(w, "%s BAD\r\n", tag)
		}
		w.Flush()
	}
}

func serveSMTP(t *testing.T, ln net.Listener) {
	t.Helper()
	c, err := ln.Accept()
	if err != nil {
		return
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	w := bufio.NewWriter(c)
	r := bufio.NewReader(c)
	fmt.Fprint(w, "220 kcrm test\r\n")
	w.Flush()
	authed := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		cmd := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
		switch cmd {
		case "EHLO", "HELO":
			fmt.Fprint(w, "250-localhost\r\n250 AUTH PLAIN LOGIN\r\n")
		case "AUTH":
			fmt.Fprint(w, "235 ok\r\n")
			authed = true
		case "QUIT":
			fmt.Fprint(w, "221 bye\r\n")
			w.Flush()
			if !authed {
				t.Error("quit without auth")
			}
			return
		default:
			fmt.Fprint(w, "250 ok\r\n")
		}
		w.Flush()
	}
}

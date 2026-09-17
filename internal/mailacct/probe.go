package mailacct

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

var probeTimeout = 8 * time.Second

func Probe(a Account, password string) error {
	if strings.TrimSpace(password) == "" {
		return ErrPassword
	}
	switch a.Kind {
	case KindIMAP:
		return probeIMAP(a, password)
	case KindSMTP:
		return probeSMTP(a, password)
	default:
		return fmt.Errorf("kind imap ou smtp")
	}
}

func dial(host string, port int, wrapTLS bool) (net.Conn, error) {
	d := net.Dialer{Timeout: probeTimeout}
	c, err := d.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("injoignable")
	}
	_ = c.SetDeadline(time.Now().Add(probeTimeout))
	if !wrapTLS {
		return c, nil
	}
	t := tls.Client(c, tlsConf(host))
	if err := t.Handshake(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("tls refuse")
	}
	return t, nil
}

func tlsConf(host string) *tls.Config {
	return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
}

func probeIMAP(a Account, password string) error {
	c, err := dial(a.Host, a.Port, a.Security == SecTLS)
	if err != nil {
		return err
	}
	defer c.Close()
	r := bufio.NewReader(c)
	if _, err := readIMAP(r); err != nil {
		return err
	}
	n := 0
	next := func() string {
		n++
		return fmt.Sprintf("a%d", n)
	}
	if a.Security == SecStart {
		tag := next()
		if err := imapCmd(c, r, tag, "STARTTLS"); err != nil {
			return err
		}
		t := tls.Client(c, tlsConf(a.Host))
		if err := t.Handshake(); err != nil {
			return fmt.Errorf("tls refuse")
		}
		_ = t.SetDeadline(time.Now().Add(probeTimeout))
		c = t
		r = bufio.NewReader(c)
	}
	tag := next()
	if err := imapCmd(c, r, tag, "LOGIN "+imapQuote(a.Username)+" "+imapQuote(password)); err != nil {
		return fmt.Errorf("login refuse")
	}
	tag = next()
	if err := imapCmd(c, r, tag, "SELECT "+imapQuote(a.Folder)); err != nil {
		return fmt.Errorf("dossier refuse")
	}
	tag = next()
	_ = imapCmd(c, r, tag, "LOGOUT")
	return nil
}

func imapCmd(c net.Conn, r *bufio.Reader, tag, cmd string) error {
	if _, err := c.Write([]byte(tag + " " + cmd + "\r\n")); err != nil {
		return fmt.Errorf("injoignable")
	}
	for {
		line, err := readIMAP(r)
		if err != nil {
			return err
		}
		if strings.HasPrefix(line, tag+" ") {
			rest := strings.TrimPrefix(line, tag+" ")
			if strings.HasPrefix(rest, "OK") {
				return nil
			}
			return fmt.Errorf("commande refusee")
		}
	}
}

func readIMAP(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("reponse imap")
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func imapQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func probeSMTP(a Account, password string) error {
	c, err := dial(a.Host, a.Port, a.Security == SecTLS)
	if err != nil {
		return err
	}
	defer c.Close()
	client, err := smtp.NewClient(c, a.Host)
	if err != nil {
		return fmt.Errorf("smtp refuse")
	}
	defer client.Close()
	if err := client.Hello("k-crm"); err != nil {
		return fmt.Errorf("smtp refuse")
	}
	if a.Security == SecStart {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("starttls absent")
		}
		if err := client.StartTLS(tlsConf(a.Host)); err != nil {
			return fmt.Errorf("tls refuse")
		}
	}
	if err := client.Auth(smtp.PlainAuth("", a.Username, password, a.Host)); err != nil {
		if err := client.Auth(loginAuth{a.Username, password}); err != nil {
			return fmt.Errorf("login refuse")
		}
	}
	_ = client.Quit()
	return nil
}

type loginAuth struct {
	user, pass string
}

func (a loginAuth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	prompt := strings.ToLower(string(fromServer))
	if strings.Contains(prompt, "user") {
		return []byte(a.user), nil
	}
	if strings.Contains(prompt, "pass") {
		return []byte(a.pass), nil
	}
	if len(fromServer) == 0 {
		return []byte(a.user), nil
	}
	return nil, fmt.Errorf("login refuse")
}

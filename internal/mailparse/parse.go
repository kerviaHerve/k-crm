package mailparse

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/mail"
	"strings"
	"time"
	"unicode"
)

const maxRaw = 1 << 20

var ErrNotMessage = errors.New("not an rfc822 message")

type Message struct {
	MessageID   string   `json:"message_id"`
	FromName    string   `json:"from_name,omitempty"`
	FromEmail   string   `json:"from_email"`
	ReplyTo     string   `json:"reply_to,omitempty"`
	To          []string `json:"to,omitempty"`
	Cc          []string `json:"cc,omitempty"`
	Subject     string   `json:"subject"`
	Date        string   `json:"date,omitempty"`
	Body        string   `json:"body"`
	Attachments []string `json:"attachments,omitempty"`
	Skip        string   `json:"skip,omitempty"`
}

func Parse(raw []byte) (Message, error) {
	if len(raw) == 0 {
		return Message{}, ErrNotMessage
	}
	if len(raw) > maxRaw {
		raw = raw[:maxRaw]
	}
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return Message{}, fmt.Errorf("%w: %v", ErrNotMessage, err)
	}
	dec := wordDecoder()
	fromName, fromEmail := firstAddr(dec, msg.Header, "From")
	if fromEmail == "" {
		return Message{}, fmt.Errorf("%w: missing From", ErrNotMessage)
	}
	out := Message{
		MessageID: cleanID(headerWord(dec, msg.Header, "Message-Id")),
		FromName:  fromName,
		FromEmail: fromEmail,
		ReplyTo:   secondEmail(dec, msg.Header, "Reply-To"),
		To:        addrList(dec, msg.Header, "To"),
		Cc:        addrList(dec, msg.Header, "Cc"),
		Subject:   strings.TrimSpace(headerWord(dec, msg.Header, "Subject")),
	}
	if t, err := msg.Header.Date(); err == nil {
		out.Date = t.UTC().Format(time.RFC3339)
	}
	if out.MessageID == "" {
		out.MessageID = "sha1:" + fingerprint(fromEmail, out.Date, out.Subject, raw)
	}
	if reason := skipReason(msg.Header, fromEmail); reason != "" {
		out.Skip = reason
		return out, nil
	}
	plain, html, atts, calendarOnly := walkMIME(msg.Body, msg.Header.Get("Content-Type"), msg.Header.Get("Content-Transfer-Encoding"))
	out.Attachments = atts
	body := strings.TrimSpace(plain)
	if body == "" {
		body = strings.TrimSpace(htmlToText(html))
	}
	body = cutSignature(body)
	body = collapseBlank(body)
	if calendarOnly && body == "" {
		out.Skip = "calendar"
		return out, nil
	}
	if len([]rune(body)) < 8 {
		out.Skip = "empty"
		return out, nil
	}
	if len([]rune(body)) > 8000 {
		body = string([]rune(body)[:8000]) + "\n…"
	}
	out.Body = body
	return out, nil
}

func Candidates(m Message) []string {
	seen := map[string]bool{}
	var out []string
	add := func(e string) {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" || seen[e] {
			return
		}
		seen[e] = true
		out = append(out, e)
	}
	add(m.FromEmail)
	add(m.ReplyTo)
	for _, e := range m.To {
		add(e)
	}
	for _, e := range m.Cc {
		add(e)
	}
	return out
}

func wordDecoder() *mime.WordDecoder {
	return &mime.WordDecoder{CharsetReader: charsetReader}
}

func headerWord(dec *mime.WordDecoder, h mail.Header, key string) string {
	v := strings.TrimSpace(h.Get(key))
	if v == "" {
		return ""
	}
	out, err := dec.DecodeHeader(v)
	if err != nil {
		return v
	}
	return out
}

func firstAddr(dec *mime.WordDecoder, h mail.Header, key string) (string, string) {
	list := parseAddrs(dec, h.Get(key))
	if len(list) == 0 {
		return "", ""
	}
	return list[0].Name, strings.ToLower(list[0].Address)
}

func secondEmail(dec *mime.WordDecoder, h mail.Header, key string) string {
	list := parseAddrs(dec, h.Get(key))
	if len(list) == 0 {
		return ""
	}
	return strings.ToLower(list[0].Address)
}

func addrList(dec *mime.WordDecoder, h mail.Header, key string) []string {
	list := parseAddrs(dec, h.Get(key))
	out := make([]string, 0, len(list))
	for _, a := range list {
		if a.Address != "" {
			out = append(out, strings.ToLower(a.Address))
		}
	}
	return out
}

func parseAddrs(dec *mime.WordDecoder, raw string) []*mail.Address {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	decoded, err := dec.DecodeHeader(raw)
	if err == nil {
		raw = decoded
	}
	p := mail.AddressParser{WordDecoder: dec}
	list, err := p.ParseList(raw)
	if err != nil {
		if a, err2 := mail.ParseAddress(raw); err2 == nil {
			return []*mail.Address{a}
		}
		return nil
	}
	return list
}

func cleanID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.Trim(id, "<>")
	return strings.TrimSpace(id)
}

func fingerprint(from, date, subject string, raw []byte) string {
	sum := sha1.Sum([]byte(from + "\n" + date + "\n" + subject + "\n" + string(raw[:min(len(raw), 4096)])))
	return hex.EncodeToString(sum[:])
}

func collapseBlank(s string) string {
	var b strings.Builder
	blank := 0
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		if strings.TrimSpace(line) == "" {
			blank++
			if blank > 1 {
				continue
			}
			b.WriteByte('\n')
			continue
		}
		blank = 0
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	b, err := io.ReadAll(io.LimitReader(input, maxRaw))
	if err != nil {
		return nil, err
	}
	return bytes.NewReader([]byte(decodeBytes(b, charset))), nil
}

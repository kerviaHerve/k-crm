package mailparse

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"strings"
	"unicode/utf8"
)

func walkMIME(r io.Reader, contentType, transfer string) (plain, html string, atts []string, calendarOnly bool) {
	body, err := decodeTransfer(r, transfer)
	if err != nil {
		return "", "", nil, false
	}
	calendarOnly = true
	var sawText bool
	var walk func(io.Reader, string, string, string)
	walk = func(src io.Reader, ct, te, disp string) {
		m, p, err := mime.ParseMediaType(ct)
		if err != nil || m == "" {
			m = "text/plain"
		}
		if strings.HasPrefix(m, "multipart/") {
			mr := multipart.NewReader(src, p["boundary"])
			for {
				part, err := mr.NextPart()
				if err != nil {
					return
				}
				walk(part, part.Header.Get("Content-Type"), part.Header.Get("Content-Transfer-Encoding"), part.Header.Get("Content-Disposition"))
			}
		}
		raw, err := decodeTransfer(src, te)
		if err != nil {
			return
		}
		name := p["name"]
		if _, dparams, err := mime.ParseMediaType(disp); err == nil && dparams["filename"] != "" {
			name = dparams["filename"]
		}
		switch {
		case m == "text/plain":
			plain += decodeBytes(raw, p["charset"])
			sawText = true
			calendarOnly = false
		case m == "text/html":
			html += decodeBytes(raw, p["charset"])
			sawText = true
			calendarOnly = false
		case m == "text/calendar":
			if name == "" {
				name = "invite.ics"
			}
			atts = append(atts, name)
		default:
			calendarOnly = false
			if name != "" {
				atts = append(atts, name)
			}
		}
	}
	walk(bytes.NewReader(body), contentType, "", "")
	if sawText {
		calendarOnly = false
	}
	return plain, html, unique(atts), calendarOnly && !sawText
}

func decodeTransfer(r io.Reader, enc string) ([]byte, error) {
	src := io.LimitReader(r, maxRaw)
	switch strings.ToLower(strings.TrimSpace(enc)) {
	case "base64":
		return io.ReadAll(base64.NewDecoder(base64.StdEncoding, src))
	case "quoted-printable":
		return io.ReadAll(quotedprintable.NewReader(src))
	default:
		return io.ReadAll(src)
	}
}

func decodeBytes(b []byte, charset string) string {
	cs := strings.ToLower(strings.TrimSpace(charset))
	cs = strings.Trim(cs, `"`)
	switch cs {
	case "", "utf-8", "utf8", "us-ascii", "ascii":
		if utf8.Valid(b) {
			return string(b)
		}
		return string(latin1(b))
	case "iso-8859-1", "latin1", "iso8859-1":
		return string(latin1(b))
	case "windows-1252", "cp1252":
		return string(win1252(b))
	default:
		if utf8.Valid(b) {
			return string(b)
		}
		return string(latin1(b))
	}
}

func latin1(b []byte) []rune {
	out := make([]rune, len(b))
	for i, c := range b {
		out[i] = rune(c)
	}
	return out
}

func win1252(b []byte) []rune {
	out := make([]rune, len(b))
	for i, c := range b {
		out[i] = win1252Rune(c)
	}
	return out
}

func win1252Rune(c byte) rune {
	switch c {
	case 0x80:
		return 0x20AC
	case 0x82:
		return 0x201A
	case 0x83:
		return 0x0192
	case 0x84:
		return 0x201E
	case 0x85:
		return 0x2026
	case 0x86:
		return 0x2020
	case 0x87:
		return 0x2021
	case 0x88:
		return 0x02C6
	case 0x89:
		return 0x2030
	case 0x8A:
		return 0x0160
	case 0x8B:
		return 0x2039
	case 0x8C:
		return 0x0152
	case 0x8E:
		return 0x017D
	case 0x91:
		return 0x2018
	case 0x92:
		return 0x2019
	case 0x93:
		return 0x201C
	case 0x94:
		return 0x201D
	case 0x95:
		return 0x2022
	case 0x96:
		return 0x2013
	case 0x97:
		return 0x2014
	case 0x98:
		return 0x02DC
	case 0x99:
		return 0x2122
	case 0x9A:
		return 0x0161
	case 0x9B:
		return 0x203A
	case 0x9C:
		return 0x0153
	case 0x9E:
		return 0x017E
	case 0x9F:
		return 0x0178
	default:
		return rune(c)
	}
}

func unique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

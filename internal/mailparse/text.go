package mailparse

import (
	"html"
	"regexp"
	"strings"
)

var (
	reScript = regexp.MustCompile(`(?is)<(script|style|head)[^>]*>.*?</(script|style|head)>`)
	reTag    = regexp.MustCompile(`(?s)<[^>]+>`)
	reBR     = regexp.MustCompile(`(?i)<br\s*/?>`)
	reBlock  = regexp.MustCompile(`(?i)</(p|div|tr|h[1-6]|li|table|blockquote)>`)
)

func htmlToText(s string) string {
	s = reScript.ReplaceAllString(s, " ")
	s = reBR.ReplaceAllString(s, "\n")
	s = reBlock.ReplaceAllString(s, "\n")
	s = reTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return strings.TrimSpace(s)
}

func cutSignature(s string) string {
	for _, sep := range []string{"\n-- \n", "\n-- \r\n", "\r\n-- \r\n"} {
		if i := strings.Index(s, sep); i >= 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}

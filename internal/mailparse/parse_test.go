package mailparse

import (
	"strings"
	"testing"
)

func TestParsePlainFilesSubjectAndBody(t *testing.T) {
	raw := "From: Lea Morel <lea@nord.example>\r\n" +
		"To: herve@net6.ch\r\n" +
		"Subject: Devis site Exonik\r\n" +
		"Message-ID: <abc@nord.example>\r\n" +
		"Date: Wed, 16 Sep 2026 10:00:00 +0200\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Bonjour Hervé,\r\n\r\nOn avance sur le devis. Peux-tu relire la page d'accueil ?\r\n\r\n-- \r\nLea Morel\r\nNord\r\n"
	m, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if m.Skip != "" {
		t.Fatalf("skip=%q", m.Skip)
	}
	if m.FromEmail != "lea@nord.example" || m.FromName != "Lea Morel" {
		t.Fatalf("from %+v", m)
	}
	if m.Subject != "Devis site Exonik" {
		t.Fatalf("subject %q", m.Subject)
	}
	if m.MessageID != "abc@nord.example" {
		t.Fatalf("id %q", m.MessageID)
	}
	if !strings.Contains(m.Body, "On avance sur le devis") {
		t.Fatalf("body %q", m.Body)
	}
	if strings.Contains(m.Body, "Nord") {
		t.Fatalf("signature kept: %q", m.Body)
	}
}

func TestSkipNoreplyListBounceAuto(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		skip string
	}{
		{"noreply", "From: noreply@shop.example\r\nSubject: Order\r\n\r\nThanks for your order, it will ship tomorrow morning.\r\n", "noreply"},
		{"list", "From: Ada <ada@list.example>\r\nList-Unsubscribe: <mailto:unsub@list.example>\r\nSubject: Weekly\r\n\r\nHello friends this is the weekly digest you never asked for.\r\n", "list"},
		{"auto", "From: Ada <ada@nord.example>\r\nAuto-Submitted: auto-replied\r\nSubject: Out of office\r\n\r\nI am away until next week please write later thanks.\r\n", "auto"},
		{"bounce", "From: Mailer-Daemon@mx.example\r\nSubject: Undelivered\r\nX-Failed-Recipients: lea@nord.example\r\n\r\nDelivery failed for this address after several tries.\r\n", "bounce"},
	}
	for _, tc := range cases {
		m, err := Parse([]byte(tc.raw))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if m.Skip != tc.skip {
			t.Fatalf("%s skip=%q want %q", tc.name, m.Skip, tc.skip)
		}
	}
}

func TestSkipEmptyAndCalendar(t *testing.T) {
	empty := "From: lea@nord.example\r\nSubject: hi\r\n\r\nOk\r\n"
	m, err := Parse([]byte(empty))
	if err != nil {
		t.Fatal(err)
	}
	if m.Skip != "empty" {
		t.Fatalf("short body skip=%q body=%q", m.Skip, m.Body)
	}
	cal := "From: lea@nord.example\r\nSubject: Meeting\r\nContent-Type: text/calendar; charset=utf-8\r\n\r\nBEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"
	m, err = Parse([]byte(cal))
	if err != nil {
		t.Fatal(err)
	}
	if m.Skip != "calendar" {
		t.Fatalf("calendar skip=%q", m.Skip)
	}
}

func TestHTMLFallbackAndAttachmentName(t *testing.T) {
	raw := "From: lea@nord.example\r\n" +
		"Subject: Page d'accueil\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=b1\r\n" +
		"\r\n" +
		"--b1\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<html><body><p>Voici le <b>devis</b> annoté.</p><script>alert(1)</script></body></html>\r\n" +
		"--b1\r\n" +
		"Content-Type: application/pdf; name=devis.pdf\r\n" +
		"Content-Disposition: attachment; filename=devis.pdf\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		"JVBERi0=\r\n" +
		"--b1--\r\n"
	m, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if m.Skip != "" {
		t.Fatalf("skip=%q body=%q", m.Skip, m.Body)
	}
	if !strings.Contains(m.Body, "devis") {
		t.Fatalf("html body %q", m.Body)
	}
	if strings.Contains(m.Body, "alert") {
		t.Fatalf("script leaked %q", m.Body)
	}
	if len(m.Attachments) != 1 || m.Attachments[0] != "devis.pdf" {
		t.Fatalf("atts %+v", m.Attachments)
	}
}

func TestRejectGarbage(t *testing.T) {
	if _, err := Parse(nil); err == nil {
		t.Fatal("empty accepted")
	}
	if _, err := Parse([]byte("hello this is not mail")); err == nil {
		t.Fatal("garbage accepted")
	}
}

func TestCandidatesFromThenTo(t *testing.T) {
	m := Message{FromEmail: "lea@nord.example", ReplyTo: "assist@nord.example", To: []string{"herve@net6.ch"}, Cc: []string{"lea@nord.example"}}
	got := Candidates(m)
	if len(got) != 3 || got[0] != "lea@nord.example" {
		t.Fatalf("%v", got)
	}
}

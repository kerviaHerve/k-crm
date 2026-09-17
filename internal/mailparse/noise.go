package mailparse

import (
	"net/mail"
	"strings"
)

func skipReason(h mail.Header, from string) string {
	as := strings.ToLower(strings.TrimSpace(h.Get("Auto-Submitted")))
	if as != "" && as != "no" {
		return "auto"
	}
	switch strings.ToLower(strings.TrimSpace(h.Get("Precedence"))) {
	case "bulk", "list", "junk":
		return "list"
	}
	if h.Get("List-Id") != "" || h.Get("List-Unsubscribe") != "" || h.Get("List-Post") != "" {
		return "list"
	}
	ct := strings.ToLower(h.Get("Content-Type"))
	if strings.Contains(ct, "multipart/report") || strings.Contains(ct, "delivery-status") {
		return "bounce"
	}
	if h.Get("X-Failed-Recipients") != "" {
		return "bounce"
	}
	low := strings.ToLower(from)
	local, _, _ := strings.Cut(low, "@")
	if strings.Contains(low, "mailer-daemon") || local == "postmaster" {
		return "bounce"
	}
	if isNoreply(local) {
		return "noreply"
	}
	return ""
}

func isNoreply(local string) bool {
	local = strings.ToLower(strings.TrimSpace(local))
	switch {
	case local == "noreply", local == "no-reply", local == "no.reply", local == "donotreply", local == "do-not-reply":
		return true
	case strings.HasPrefix(local, "noreply"), strings.HasPrefix(local, "no-reply"), strings.HasPrefix(local, "no.reply"):
		return true
	case strings.Contains(local, "donotreply"):
		return true
	case local == "notifications", local == "notify", local == "notification", local == "alerts", local == "alert":
		return true
	default:
		return false
	}
}

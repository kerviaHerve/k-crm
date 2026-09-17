package version

import (
	"runtime/debug"
	"strings"
)

const Number = "0.1.0-beta"

// Revision is the git SHA, set by ldflags or discovered from build VCS info.
var Revision string

func init() {
	if Revision != "" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	var rev string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return
	}
	if dirty {
		Revision = rev + "-dirty"
	} else {
		Revision = rev
	}
}

func ShortRev() string {
	r := strings.TrimSpace(Revision)
	dirty := strings.HasSuffix(r, "-dirty")
	r = strings.TrimSuffix(r, "-dirty")
	if len(r) > 7 {
		r = r[:7]
	}
	if r != "" && dirty {
		return r + "-dirty"
	}
	return r
}

func Line() string {
	if r := ShortRev(); r != "" {
		return "k-crm " + Number + " (" + r + ")"
	}
	return "k-crm " + Number
}

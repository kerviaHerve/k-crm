package version

import (
	"strings"
	"testing"
)

func TestLineContainsNumber(t *testing.T) {
	if !strings.Contains(Line(), Number) {
		t.Fatalf("line %q", Line())
	}
}

func TestShortRev(t *testing.T) {
	old := Revision
	t.Cleanup(func() { Revision = old })
	Revision = "abcdef123456"
	if ShortRev() != "abcdef1" {
		t.Fatalf("got %q", ShortRev())
	}
	Revision = "abc"
	if ShortRev() != "abc" {
		t.Fatalf("got %q", ShortRev())
	}
	Revision = "abcdef123-dirty"
	if ShortRev() != "abcdef1-dirty" {
		t.Fatalf("got %q", ShortRev())
	}
}

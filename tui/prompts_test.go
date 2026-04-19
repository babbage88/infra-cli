package tui

import (
	"strings"
	"testing"
)

func TestCleanTerminalLogTextRemovesTerminalControlSequences(t *testing.T) {
	input := "Get:1 package [2087 kB]\r\n(Reading database ... 15%\r\x1b[K(Reading database ... 35%\r\nDone\x1b[0m\n"
	got := CleanTerminalLogText(input)

	for _, disallowed := range []string{"\r", "\x1b", "\x00"} {
		if strings.Contains(got, disallowed) {
			t.Fatalf("cleaned log still contains %q: %q", disallowed, got)
		}
	}

	for _, want := range []string{
		"Get:1 package [2087 kB]",
		"(Reading database ... 35%",
		"Done",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("cleaned log missing %q: %q", want, got)
		}
	}
	if strings.Contains(got, "(Reading database ... 15%") {
		t.Fatalf("cleaned log should collapse overwritten progress lines: %q", got)
	}
}

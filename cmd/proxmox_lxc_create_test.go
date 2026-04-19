package cmd

import (
	"errors"
	"strings"
	"testing"
)

func TestRenderLxcSSHForceCommandEntriesDoesNotEmitRawCarriageReturns(t *testing.T) {
	rendered := renderLxcSSHForceCommandEntries([]lxcSSHForceLogEntry{{
		Kind:  lxcSSHForceLogCommand,
		Label: "pct exec 252\r",
		Body:  "Preparing\r(Reading database ... 55%\r\x1b[KProcessing triggers...\n",
	}}, 80)

	if strings.Contains(rendered, "\r") {
		t.Fatalf("rendered command log contains raw terminal controls: %q", rendered)
	}
	if !strings.Contains(rendered, "Processing triggers...") {
		t.Fatalf("rendered command log missing expected content: %q", rendered)
	}
}

func TestLxcSSHForcePromptErrorIsSingleLineAndCompact(t *testing.T) {
	err := errors.New("ensure SSH server failed: SSH execution failed: process exited with status 255: line one\nline two\nline three")

	rendered := lxcSSHForcePromptError(err, 96)

	if strings.Contains(rendered, "\n") {
		t.Fatalf("prompt error contains newline: %q", rendered)
	}
	if !strings.Contains(rendered, "See stdout/stderr above.") {
		t.Fatalf("prompt error missing log hint: %q", rendered)
	}
	if len(rendered) > 96 {
		t.Fatalf("prompt error was not compacted: len=%d value=%q", len(rendered), rendered)
	}
}

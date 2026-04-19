package cmd

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/babbage88/infra-cli/proxmox"
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

func TestPrintLxcCreateConnectionInfoShowsSSHWithoutGeneratedPassword(t *testing.T) {
	output := captureStdout(t, func() {
		printLxcCreateConnectionInfo(&proxmox.LxcContainer{VmId: 256, Hostname: "web-01"}, lxcCreateResultInfo{
			IPv4Address: "10.0.1.38",
		})
	})

	if !strings.Contains(output, "SSH: ssh root@10.0.1.38") {
		t.Fatalf("connection info missing SSH instruction: %q", output)
	}
	if strings.Contains(output, "Root password:") {
		t.Fatalf("connection info printed an empty root password line: %q", output)
	}
}

func TestPrintLxcCreateConnectionInfoUsesAdminUserForSSH(t *testing.T) {
	output := captureStdout(t, func() {
		printLxcCreateConnectionInfo(&proxmox.LxcContainer{VmId: 257, Hostname: "appdev007"}, lxcCreateResultInfo{
			IPv4Address: "10.0.1.39",
			SSHUser:     "jtrahan",
		})
	})

	if !strings.Contains(output, "SSH: ssh jtrahan@10.0.1.39") {
		t.Fatalf("connection info missing admin SSH instruction: %q", output)
	}
	if strings.Contains(output, "SSH: ssh root@10.0.1.39") {
		t.Fatalf("connection info still printed root SSH instruction: %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = originalStdout
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(out)
}

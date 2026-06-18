package cmd

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/babbage88/infra-cli/proxmox"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

func TestResolveProxmoxAPIHostForNodePrefersDerivedNodeURLOverRootDefault(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("host-url", "", "")

	vp := viper.New()
	vp.Set("host_url", "https://proxmox3:8006")

	got := resolveProxmoxAPIHostForNode(cmd, vp, "proxmox1", "https://proxmox3:8006")
	want := "https://proxmox1:8006"

	if got != want {
		t.Fatalf("resolveProxmoxAPIHostForNode() = %q, want %q", got, want)
	}
}

func TestResolveProxmoxAPIHostForNodeKeepsExplicitHostURL(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("host-url", "", "")
	if err := cmd.Flags().Set("host-url", "https://proxmox3:8006"); err != nil {
		t.Fatalf("set host-url flag: %v", err)
	}

	vp := viper.New()
	got := resolveProxmoxAPIHostForNode(cmd, vp, "proxmox1", "https://proxmox3:8006")
	want := "https://proxmox3:8006"

	if got != want {
		t.Fatalf("resolveProxmoxAPIHostForNode() = %q, want %q", got, want)
	}
}

func TestExplainLxcCreateErrorAdds401Hint(t *testing.T) {
	err := explainLxcCreateError("https://proxmox1:8006", "proxmox1", &proxmox.APIError{Status: 401})
	got := err.Error()

	if !strings.Contains(got, "401 Unauthorized") {
		t.Fatalf("expected 401 hint in error: %q", got)
	}
	if !strings.Contains(got, "https://proxmox1:8006") {
		t.Fatalf("expected host URL in error: %q", got)
	}
	if !strings.Contains(got, "token/secret") {
		t.Fatalf("expected actionable auth hint in error: %q", got)
	}
}

func TestFallbackTemplateParsingSkipsNoiseLines(t *testing.T) {
	output := "tput: No value for $TERM and no -T specified\npbs1: error fetching datastores - 500 Can't connect\nlocal:vztmpl/debian-13-standard_13.1-2_amd64.tar.zst\n"

	lines := strings.Split(strings.TrimSpace(output), "\n")
	templates := make([]string, 0, len(lines))
	seen := make(map[string]struct{})
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ":vztmpl/") {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		templates = append(templates, line)
	}

	if len(templates) != 1 || templates[0] != "local:vztmpl/debian-13-standard_13.1-2_amd64.tar.zst" {
		t.Fatalf("unexpected templates parsed from noisy output: %v", templates)
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

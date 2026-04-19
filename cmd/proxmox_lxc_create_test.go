package cmd

import (
	"strings"
	"testing"
)

func TestLxcSSHForceBootstrapScriptUsesStableLocaleForApt(t *testing.T) {
	script := lxcSSHForceBootstrapScript([]string{"ssh-ed25519 AAAATEST user@example"}, lxcSSHForceOptions{})

	for _, want := range []string{
		"export LC_ALL=C",
		"export LANG=C",
		"export LANGUAGE=C",
		"DEBIAN_FRONTEND=noninteractive APT_LISTCHANGES_FRONTEND=none apt-get install -y \"$@\"",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("bootstrap script missing %q\nscript:\n%s", want, script)
		}
	}
}

func TestLxcSSHForceBootstrapScriptIncludesAdminSetup(t *testing.T) {
	script := lxcSSHForceBootstrapScript([]string{"ssh-ed25519 AAAATEST user@example"}, lxcSSHForceOptions{
		AddAdminUser: true,
		AdminUser:    "deploy",
		AdminUID:     1001,
	})

	for _, want := range []string{
		"install_pkg sudo",
		"admin_user='deploy'",
		"admin_uid='1001'",
		"printf '%s ALL=(ALL) NOPASSWD:ALL\\n' \"$admin_user\" > '/etc/sudoers.d/90-infractl-deploy'",
		"ensure_authorized_keys \"$admin_user\" \"$admin_home\"",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("bootstrap script missing %q\nscript:\n%s", want, script)
		}
	}
}

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

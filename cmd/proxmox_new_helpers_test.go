package cmd

import (
	"errors"
	"testing"
)

type fakeProxmoxSSHClient struct {
	command  string
	commands []string
	outputs  [][]byte
	errs     []error
	calls    int
}

func (f *fakeProxmoxSSHClient) Run(cmd string) ([]byte, error) {
	f.command = cmd
	f.commands = append(f.commands, cmd)
	if f.calls < len(f.outputs) || f.calls < len(f.errs) {
		var out []byte
		if f.calls < len(f.outputs) {
			out = f.outputs[f.calls]
		}
		var err error
		if f.calls < len(f.errs) {
			err = f.errs[f.calls]
		}
		f.calls++
		return out, err
	}
	f.calls++
	return []byte("ok"), nil
}

func (f *fakeProxmoxSSHClient) Upload(_, _ string) error {
	return nil
}

func (f *fakeProxmoxSSHClient) Close() error {
	return nil
}

func TestRunRemoteQuotedCommandSetsDefaultTERM(t *testing.T) {
	client := &fakeProxmoxSSHClient{}

	if _, err := runRemoteQuotedCommand(client, "pveum", "user", "token", "permissions", "root@pam!infractl-cli"); err != nil {
		t.Fatalf("run remote quoted command: %v", err)
	}

	want := `TERM="${TERM:-dumb}" 'pveum' 'user' 'token' 'permissions' 'root@pam!infractl-cli'`
	if client.command != want {
		t.Fatalf("unexpected command:\nwant: %s\ngot:  %s", want, client.command)
	}
}

func TestProxmoxUserExistsOverSSHExtractsJSONArrayFromNoisyOutput(t *testing.T) {
	client := &fakeProxmoxSSHClient{
		outputs: [][]byte{
			[]byte("prefix noise\n[{\"userid\":\"infractl@pve\"}]\n"),
		},
	}

	exists, err := proxmoxUserExistsOverSSH(client, "infractl@pve")
	if err != nil {
		t.Fatalf("proxmoxUserExistsOverSSH returned error: %v", err)
	}
	if !exists {
		t.Fatalf("expected proxmox user to be detected from noisy JSON output")
	}
}

func TestProxmoxUserExistsOverSSHFallsBackToPlainList(t *testing.T) {
	client := &fakeProxmoxSSHClient{
		outputs: [][]byte{
			[]byte("plain output, not json"),
			[]byte("userid comment\ninfractl@pve Created by infractl\n"),
		},
	}

	exists, err := proxmoxUserExistsOverSSH(client, "infractl@pve")
	if err != nil {
		t.Fatalf("proxmoxUserExistsOverSSH returned error: %v", err)
	}
	if !exists {
		t.Fatalf("expected proxmox user to be detected from fallback plain output")
	}
}

func TestProxmoxUserExistsOverSSHFallsBackAfterJSONCommandError(t *testing.T) {
	client := &fakeProxmoxSSHClient{
		outputs: [][]byte{
			nil,
			[]byte("userid comment\ninfractl@pve Created by infractl\n"),
		},
		errs: []error{
			errors.New("json mode unavailable"),
			nil,
		},
	}

	exists, err := proxmoxUserExistsOverSSH(client, "infractl@pve")
	if err != nil {
		t.Fatalf("proxmoxUserExistsOverSSH returned error: %v", err)
	}
	if !exists {
		t.Fatalf("expected proxmox user to be detected after fallback from command error")
	}
}

func TestAssignRoleToProxmoxPrincipalOverSSHAddsPropagationForUser(t *testing.T) {
	client := &fakeProxmoxSSHClient{}

	if err := assignRoleToProxmoxPrincipalOverSSH(client, "/", "user", "infractl@pve", infraCtlManagerRoleName); err != nil {
		t.Fatalf("assignRoleToProxmoxPrincipalOverSSH returned error: %v", err)
	}

	want := `TERM="${TERM:-dumb}" 'pveum' 'aclmod' '/' '-role' 'InfraCtlProxmoxManager' '-propagate' '1' '-user' 'infractl@pve'`
	if client.command != want {
		t.Fatalf("unexpected command:\nwant: %s\ngot:  %s", want, client.command)
	}
}

func TestAssignRoleToProxmoxPrincipalOverSSHAddsPropagationForToken(t *testing.T) {
	client := &fakeProxmoxSSHClient{}

	if err := assignRoleToProxmoxPrincipalOverSSH(client, "/", "token", "infractl@pve!infractl-cli", infraCtlManagerRoleName); err != nil {
		t.Fatalf("assignRoleToProxmoxPrincipalOverSSH returned error: %v", err)
	}

	want := `TERM="${TERM:-dumb}" 'pveum' 'aclmod' '/' '-role' 'InfraCtlProxmoxManager' '-propagate' '1' '-token' 'infractl@pve!infractl-cli'`
	if client.command != want {
		t.Fatalf("unexpected command:\nwant: %s\ngot:  %s", want, client.command)
	}
}

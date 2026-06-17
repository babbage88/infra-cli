package cmd

import "testing"

type fakeProxmoxSSHClient struct {
	command string
}

func (f *fakeProxmoxSSHClient) Run(cmd string) ([]byte, error) {
	f.command = cmd
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

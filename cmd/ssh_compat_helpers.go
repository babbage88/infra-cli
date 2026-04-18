package cmd

import infraSSH "github.com/babbage88/infra-cli/ssh"

func expandPath(path string) string {
	return infraSSH.ExpandPath(path)
}

func currentUserName() string {
	return infraSSH.CurrentUserName()
}

func defaultSSHKeyPath() string {
	return infraSSH.DefaultPrivateKeyPath()
}

func shellQuote(s string) string {
	return infraSSH.ShellQuote(s)
}

func formatSSHExecError(err error, out []byte) error {
	return infraSSH.FormatExecError(err, out)
}

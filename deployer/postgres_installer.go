package deployer

import (
	"bufio"
	"os"
	"strings"

	"github.com/babbage88/goph/v2"
	"github.com/babbage88/infra-cli/ssh"
)

type RemotePostgresInstaller struct {
	RemoteHostname string       `json:"hostname"`
	SshClient      *goph.Client `json:"sshClient"`
	RemoteSshUser  string       `json:"remoteSshUser"`
}

func NewRemotePostgresInstallerWithSsh(hostname, sshUser, sshKey, sshPassphrase string, useSshAgent bool, port uint) (*RemotePostgresInstaller, error) {
	client, err := ssh.InitializeSshClient(hostname, sshUser, sshKey, sshPassphrase, useSshAgent, port)
	if err != nil {
		return nil, err
	}
	pgInstaller := &RemotePostgresInstaller{
		SshClient:      client,
		RemoteHostname: hostname,
		RemoteSshUser:  sshUser,
	}
	return pgInstaller, nil
}

func ReadEnvFile(filePath string) (map[string]string, error) {
	envMap := make(map[string]string)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		envMap[key] = value
	}

	if scanner.Err() != nil {
		return nil, scanner.Err()
	}

	return envMap, nil
}

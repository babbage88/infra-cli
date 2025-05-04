package deployer

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/babbage88/goph/v2"
	"github.com/babbage88/infra-cli/ssh"
)

type RemotePostgresInstaller struct {
	RemoteHostname string            `json:"hostname"`
	SshClient      *goph.Client      `json:"sshClient"`
	RemoteSshUser  string            `json:"remoteSshUser"`
	OsInfo         map[string]string `json:"osInfo"`
	PgPassword     string
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

func (rpg *RemotePostgresInstaller) ReadOsReleaseFile(filePath string) error {
	osInfoMap := make(map[string]string)

	file, err := os.Open(filePath)
	if err != nil {
		return err
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
		osInfoMap[key] = value
	}

	if scanner.Err() != nil {
		return scanner.Err()
	}

	rpg.OsInfo = osInfoMap
	return nil
}

func (rpg *RemotePostgresInstaller) ParseOsReleaseContent(output []byte) error {
	osInfoMap := make(map[string]string)

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes if present
		value = strings.Trim(value, `"'`)

		osInfoMap[key] = value
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	rpg.OsInfo = osInfoMap
	return nil
}

func (rpg *RemotePostgresInstaller) InstallApplication() error {
	if rpg.OsInfo == nil {
		out, err := rpg.SshClient.Run("cat /etc/os-release")
		if err != nil {
			slog.Error("Error parsing /etc/os-release")
			return err
		}
		rpg.ParseOsReleaseContent(out)
	}

	osId := rpg.OsInfo["ID"]
	osIdLike := rpg.OsInfo["ID_LIKE"]
	rh_like := osIdLike == "rhel fedora" || osId == "fedora" || osId == "centos" || osId == "rocky" || osId == "alma"
	debian_like := osIdLike == "debian" || osId == "ubuntu" || osId == "debian"
	arch_like := osIdLike == "arch" || osId == "arch"
	supportedOs := rh_like || debian_like || arch_like

	if !supportedOs {
		return fmt.Errorf("unsupported OS: %s", osId)
	}

	switch {
	case debian_like:
		// Install PostgreSQL
		_, err := rpg.SshClient.Run("sudo apt-get update -y && sudo apt-get install -y postgresql")
		if err != nil {
			return fmt.Errorf("failed to install PostgreSQL: %w", err)
		}

		// Change postgres user password
		changePassCmd := fmt.Sprintf(`sudo -u postgres psql -c "ALTER USER postgres WITH PASSWORD '%s';"`, rpg.PgPassword)
		_, err = rpg.SshClient.Run(changePassCmd)
		if err != nil {
			return fmt.Errorf("failed to change postgres user password: %w", err)
		}

		// Locate PostgreSQL config files
		confDirCmd := `find /etc/postgresql /etc/postgresql/*/main /var/lib/pgsql /var/lib/postgres -type f \( -name "postgresql.conf" -o -name "pg_hba.conf" \) 2>/dev/null`
		confPathsRaw, err := rpg.SshClient.Run(confDirCmd)
		if err != nil {
			return fmt.Errorf("failed to find PostgreSQL config files: %w", err)
		}

		confPathsStr := string(confPathsRaw)
		var postgresqlConfPath, pgHbaConfPath string
		for _, line := range strings.Split(confPathsStr, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasSuffix(line, "postgresql.conf") {
				postgresqlConfPath = line
			} else if strings.HasSuffix(line, "pg_hba.conf") {
				pgHbaConfPath = line
			}
		}

		if postgresqlConfPath == "" || pgHbaConfPath == "" {
			return fmt.Errorf("could not locate postgresql.conf or pg_hba.conf")
		}

		// Set listen_addresses = '*'
		_, err = rpg.SshClient.Run(fmt.Sprintf(`sudo sed -i "s/^#*listen_addresses.*/listen_addresses = '*'/g" %s`, postgresqlConfPath))
		if err != nil {
			return fmt.Errorf("failed to set listen_addresses: %w", err)
		}

		// Set password_encryption = 'scram-sha-256'
		_, err = rpg.SshClient.Run(fmt.Sprintf(`sudo sed -i "s/^#*password_encryption.*/password_encryption = 'scram-sha-256'/g" %s`, postgresqlConfPath))
		if err != nil {
			return fmt.Errorf("failed to set password_encryption: %w", err)
		}

		// Add pg_hba.conf rule for scram-sha-256 auth
		_, err = rpg.SshClient.Run(fmt.Sprintf(`echo "host all all 0.0.0.0/0 scram-sha-256" | sudo tee -a %s`, pgHbaConfPath))
		if err != nil {
			return fmt.Errorf("failed to append scram-sha-256 rule to pg_hba.conf: %w", err)
		}

		// Restart PostgreSQL
		_, err = rpg.SshClient.Run("sudo systemctl restart postgresql")
		if err != nil {
			return fmt.Errorf("failed to restart PostgreSQL: %w", err)
		}
	}

	return nil
}

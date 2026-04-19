package deployer

import (
	"fmt"
	"path/filepath"
	"strings"

	infraSSH "github.com/babbage88/infra-cli/ssh"
)

func ConfigureRemotePostgresAccess(sshClient infraSSH.Client, hbaCIDR, authMethod, listenAddresses string) error {
	findCmd := `find /etc/postgresql /etc/postgresql/*/main /var/lib/pgsql /var/lib/postgresql /var/lib/postgres -type f \( -name "postgresql.conf" -o -name "pg_hba.conf" \) 2>/dev/null`
	out, err := sshClient.Run(findCmd)

	var postgresqlConfPath, pgHbaConfPath string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasSuffix(line, "postgresql.conf") && postgresqlConfPath == "":
			postgresqlConfPath = line
		case strings.HasSuffix(line, "pg_hba.conf") && pgHbaConfPath == "":
			pgHbaConfPath = line
		}
	}

	if err != nil && (postgresqlConfPath == "" || pgHbaConfPath == "") {
		return formatRemoteCommandError(fmt.Errorf("find postgres config files: %w", err), out)
	}

	if postgresqlConfPath == "" || pgHbaConfPath == "" {
		return fmt.Errorf("could not locate postgresql.conf or pg_hba.conf on remote host")
	}

	listenScript := fmt.Sprintf(
		`if grep -Eq "^[#[:space:]]*listen_addresses[[:space:]]*=" %s; then sed -i "s/^[#[:space:]]*listen_addresses[[:space:]]*=.*/listen_addresses = %s/" %s; else printf "\nlisten_addresses = %s\n" >> %s; fi`,
		shellQuote(postgresqlConfPath),
		shellQuote(listenAddresses),
		shellQuote(postgresqlConfPath),
		shellQuote(listenAddresses),
		shellQuote(postgresqlConfPath),
	)
	if out, err = sshClient.Run("sudo sh -c " + shellQuote(listenScript)); err != nil {
		return formatRemoteCommandError(fmt.Errorf("configure listen_addresses: %w", err), out)
	}

	passwordEncScript := fmt.Sprintf(
		`if grep -Eq "^[#[:space:]]*password_encryption[[:space:]]*=" %s; then sed -i "s/^[#[:space:]]*password_encryption[[:space:]]*=.*/password_encryption = %s/" %s; else printf "\npassword_encryption = %s\n" >> %s; fi`,
		shellQuote(postgresqlConfPath),
		shellQuote("scram-sha-256"),
		shellQuote(postgresqlConfPath),
		shellQuote("scram-sha-256"),
		shellQuote(postgresqlConfPath),
	)
	if out, err = sshClient.Run("sudo sh -c " + shellQuote(passwordEncScript)); err != nil {
		return formatRemoteCommandError(fmt.Errorf("configure password_encryption: %w", err), out)
	}

	hbaRule := fmt.Sprintf("host all all %s %s", hbaCIDR, authMethod)
	hbaScript := fmt.Sprintf(
		`grep -Fqx %s %s || printf "\n%s\n" >> %s`,
		shellQuote(hbaRule),
		shellQuote(pgHbaConfPath),
		hbaRule,
		shellQuote(pgHbaConfPath),
	)
	if out, err = sshClient.Run("sudo sh -c " + shellQuote(hbaScript)); err != nil {
		return formatRemoteCommandError(fmt.Errorf("update pg_hba.conf: %w", err), out)
	}

	return restartRemotePostgresService(sshClient, postgresqlConfPath)
}

func restartRemotePostgresService(sshClient infraSSH.Client, postgresqlConfPath string) error {
	dataDir := filepath.ToSlash(filepath.Dir(postgresqlConfPath))
	versionCandidate := ""
	parts := strings.Split(dataDir, "/")
	for i, part := range parts {
		if part == "pgsql" && i+1 < len(parts) && parts[i+1] != "" {
			versionCandidate = parts[i+1]
			break
		}
	}

	candidates := []string{"postgresql", "postgresql.service"}
	if versionCandidate != "" {
		candidates = append(candidates, "postgresql-"+versionCandidate, "postgresql-"+versionCandidate+".service")
	}

	restartScript := fmt.Sprintf(
		`set -e
for svc in %s $(systemctl list-unit-files 'postgresql*' --type=service --no-legend 2>/dev/null | awk '{print $1}') $(systemctl list-units --all 'postgresql*' --type=service --no-legend 2>/dev/null | awk '{print $1}'); do
  [ -n "$svc" ] || continue
  if sudo systemctl restart "$svc" >/dev/null 2>&1; then
    exit 0
  fi
done
if command -v pg_ctl >/dev/null 2>&1; then
  sudo -u postgres pg_ctl -D %s restart >/dev/null 2>&1 && exit 0
fi
if command -v service >/dev/null 2>&1; then
  sudo service postgresql restart >/dev/null 2>&1 && exit 0
fi
echo "Unable to restart PostgreSQL service. Tried systemd units and pg_ctl for data dir %s." >&2
exit 1`,
		shellJoinWords(candidates),
		shellQuote(dataDir),
		dataDir,
	)

	out, err := sshClient.Run("sh -c " + shellQuote(restartScript))
	if err != nil {
		return formatRemoteCommandError(fmt.Errorf("restart postgresql service: %w", err), out)
	}
	return nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func shellJoinWords(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		quoted = append(quoted, shellQuote(value))
	}
	return strings.Join(quoted, " ")
}

func formatRemoteCommandError(err error, out []byte) error {
	output := strings.TrimSpace(string(out))
	if output == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, output)
}

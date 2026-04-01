package cmd

import (
	"bufio"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/babbage88/goph/v2"
	"github.com/babbage88/infra-cli/ssh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/lib/pq"
	"golang.org/x/term"
)

// expandPath expands ~ to the user's home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// execSQLViaSsh executes a SQL statement on a remote PostgreSQL instance via SSH with sudo
func execSQLViaSsh(sshClient *goph.Client, pgUser, dbname, stmt string) error {
	cmdStr := buildRemotePsqlCommand(pgUser, dbname, stmt, false)

	out, err := sshClient.Run(cmdStr)
	if err != nil {
		return formatSSHExecError(err, out)
	}
	return nil
}

func execSQLViaSshBool(sshClient *goph.Client, pgUser, dbname, stmt string) (bool, error) {
	cmdStr := buildRemotePsqlCommand(pgUser, dbname, stmt, true)

	out, err := sshClient.Run(cmdStr)
	if err != nil {
		return false, formatSSHExecError(err, out)
	}

	switch strings.TrimSpace(string(out)) {
	case "t", "true", "1":
		return true, nil
	case "f", "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("unexpected boolean query result: %q", strings.TrimSpace(string(out)))
	}
}

func buildRemotePsqlCommand(pgUser, dbname, stmt string, tuplesOnly bool) string {
	args := []string{
		"sudo", "-u", pgUser,
		"psql",
		"-v", "ON_ERROR_STOP=1",
		"-d", dbname,
	}

	if tuplesOnly {
		args = append(args, "-tA")
	}

	args = append(args, "-c", stmt)

	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}

	return strings.Join(quoted, " ")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func formatSSHExecError(err error, out []byte) error {
	output := strings.TrimSpace(string(out))
	if output == "" {
		return fmt.Errorf("SSH execution failed: %w", err)
	}
	return fmt.Errorf("SSH execution failed: %w: %s", err, output)
}

func currentUserName() string {
	if username := os.Getenv("USER"); username != "" {
		return username
	}

	curUser, err := user.Current()
	if err == nil && curUser.Username != "" {
		return curUser.Username
	}

	return "root"
}

func defaultSSHKeyPath() string {
	for _, candidate := range []string{"~/.ssh/id_ed25519", "~/.ssh/id_rsa"} {
		expanded := expandPath(candidate)
		if info, err := os.Stat(expanded); err == nil && !info.IsDir() {
			return expanded
		}
	}

	return ""
}

func promptInput(label, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)

	for {
		if defaultValue != "" {
			fmt.Printf("%s [%s]: ", label, defaultValue)
		} else {
			fmt.Printf("%s: ", label)
		}

		input, err := reader.ReadString('\n')
		if err != nil {
			slog.Error("Failed to read input", "error", err.Error())
			os.Exit(1)
		}

		input = strings.TrimSpace(input)
		if input == "" && defaultValue != "" {
			return defaultValue
		}
		if input != "" {
			return input
		}
	}
}

func promptPassword(label, defaultValue string) string {
	for {
		if defaultValue != "" {
			fmt.Printf("%s [press enter to use current default]: ", label)
		} else {
			fmt.Printf("%s: ", label)
		}

		var input string
		if term.IsTerminal(int(os.Stdin.Fd())) {
			raw, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				slog.Error("Failed to read password", "error", err.Error())
				os.Exit(1)
			}
			input = strings.TrimSpace(string(raw))
		} else {
			reader := bufio.NewReader(os.Stdin)
			raw, err := reader.ReadString('\n')
			if err != nil {
				slog.Error("Failed to read password", "error", err.Error())
				os.Exit(1)
			}
			input = strings.TrimSpace(raw)
		}

		if input == "" && defaultValue != "" {
			return defaultValue
		}
		if input != "" {
			return input
		}
	}
}

func promptYesNo(label string, defaultYes bool) bool {
	reader := bufio.NewReader(os.Stdin)
	defaultLabel := "y/N"
	if defaultYes {
		defaultLabel = "Y/n"
	}

	for {
		fmt.Printf("%s [%s]: ", label, defaultLabel)
		input, err := reader.ReadString('\n')
		if err != nil {
			slog.Error("Failed to read input", "error", err.Error())
			os.Exit(1)
		}

		switch strings.ToLower(strings.TrimSpace(input)) {
		case "":
			return defaultYes
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
	}
}

func promptForMissingAppConfig(cmd *cobra.Command, dbname, username, password string) (string, string, string) {
	if !cmd.Flags().Changed("db-name") {
		dbname = promptInput("Database name", dbname)
	}
	if !cmd.Flags().Changed("db-user") {
		username = promptInput("Database user", username)
	}
	if !cmd.Flags().Changed("db-password") {
		password = promptPassword("Database password", password)
	}

	return dbname, username, password
}

func dropAndRecreateDatabaseViaSSH(sshClient *goph.Client, pgUser, dbname string) error {
	statements := []string{
		fmt.Sprintf(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = %s AND pid <> pg_backend_pid();`, pq.QuoteLiteral(dbname)),
		fmt.Sprintf(`DROP DATABASE IF EXISTS %s;`, pq.QuoteIdentifier(dbname)),
		fmt.Sprintf(`CREATE DATABASE %s WITH OWNER = postgres ENCODING = %s TEMPLATE = template0;`, pq.QuoteIdentifier(dbname), pq.QuoteLiteral("UTF8")),
	}

	for _, stmt := range statements {
		if err := execSQLViaSsh(sshClient, pgUser, "postgres", stmt); err != nil {
			return err
		}
	}

	return nil
}

func dropAndRecreateDatabaseDirect(db *sql.DB, dbname string) error {
	statements := []string{
		fmt.Sprintf(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = %s AND pid <> pg_backend_pid();`, pq.QuoteLiteral(dbname)),
		fmt.Sprintf(`DROP DATABASE IF EXISTS %s;`, pq.QuoteIdentifier(dbname)),
		fmt.Sprintf(`CREATE DATABASE %s WITH OWNER = postgres ENCODING = %s TEMPLATE = template0;`, pq.QuoteIdentifier(dbname), pq.QuoteLiteral("UTF8")),
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}

func buildPostgresURL(host string, port int, dbname, username, password string) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		urlQueryEscape(username),
		urlQueryEscape(password),
		host,
		port,
		urlQueryEscape(dbname),
	)
}

func urlQueryEscape(value string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		":", "%3A",
		"/", "%2F",
		"?", "%3F",
		"#", "%23",
		"[", "%5B",
		"]", "%5D",
		"@", "%40",
	)
	return replacer.Replace(value)
}

func runGooseyBinary(gooseyPath, dbURL string) error {
	gooseyPath = expandPath(gooseyPath)
	if gooseyPath == "" {
		return nil
	}

	if info, err := os.Stat(gooseyPath); err != nil {
		return fmt.Errorf("stat goosey binary %q: %w", gooseyPath, err)
	} else if info.IsDir() {
		return fmt.Errorf("goosey path %q is a directory", gooseyPath)
	}

	cmd := exec.Command(gooseyPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(
		os.Environ(),
		"DATABASE_URL="+dbURL,
		"GOOSE_DBSTRING="+dbURL,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run goosey binary %q: %w", gooseyPath, err)
	}

	return nil
}

func maybeBuildRemoteGooseyBinary(gooseyPath string, buildRemote bool, goos, goarch string) (string, func(), error) {
	gooseyPath = expandPath(gooseyPath)
	if gooseyPath == "" || !buildRemote {
		return gooseyPath, func() {}, nil
	}

	sourceDir := filepath.Dir(gooseyPath)
	info, err := os.Stat(sourceDir)
	if err != nil {
		return "", nil, fmt.Errorf("stat goosey source directory %q: %w", sourceDir, err)
	}
	if !info.IsDir() {
		return "", nil, fmt.Errorf("goosey source directory %q is not a directory", sourceDir)
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("goosey-%s-%s-*", goos, goarch))
	if err != nil {
		return "", nil, fmt.Errorf("create temp goosey binary: %w", err)
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", nil, fmt.Errorf("close temp goosey binary: %w", err)
	}

	buildCmd := exec.Command("go", "build", "-o", tmpPath, ".")
	buildCmd.Dir = sourceDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	buildCmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")

	if err := buildCmd.Run(); err != nil {
		os.Remove(tmpPath)
		return "", nil, fmt.Errorf("build remote goosey binary for %s/%s from %q: %w", goos, goarch, sourceDir, err)
	}

	cleanup := func() {
		_ = os.Remove(tmpPath)
	}

	return tmpPath, cleanup, nil
}

func runGooseyBinaryRemote(sshClient *goph.Client, gooseyPath, dbURL string) error {
	gooseyPath = expandPath(gooseyPath)
	if gooseyPath == "" {
		return nil
	}

	info, err := os.Stat(gooseyPath)
	if err != nil {
		return fmt.Errorf("stat goosey binary %q: %w", gooseyPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("goosey path %q is a directory", gooseyPath)
	}

	remotePath := filepath.ToSlash(filepath.Join("/tmp", fmt.Sprintf("goosey-%d", os.Getpid())))
	if err := sshClient.Upload(gooseyPath, remotePath); err != nil {
		return fmt.Errorf("upload goosey binary to remote host: %w", err)
	}

	cleanupCmd := fmt.Sprintf("rm -f %s", shellQuote(remotePath))
	defer func() {
		if _, cleanupErr := sshClient.Run(cleanupCmd); cleanupErr != nil {
			slog.Warn("Failed to remove remote goosey binary", "path", remotePath, "error", cleanupErr.Error())
		}
	}()

	cmdStr := fmt.Sprintf(
		"chmod 755 %s && env DATABASE_URL=%s GOOSE_DBSTRING=%s %s",
		shellQuote(remotePath),
		shellQuote(dbURL),
		shellQuote(dbURL),
		shellQuote(remotePath),
	)

	out, err := sshClient.Run(cmdStr)
	if err != nil {
		output := strings.TrimSpace(string(out))
		if strings.Contains(output, "cannot execute binary file") {
			return fmt.Errorf(
				"remote goosey binary is not executable on the target host; this usually means it was built for the wrong OS/architecture. Build a Linux binary for the remote host and try again: %s",
				output,
			)
		}
		return formatSSHExecError(fmt.Errorf("run remote goosey binary: %w", err), out)
	}

	return nil
}

func setupRemotePostgresAccess(sshClient *goph.Client, hbaCIDR, authMethod, listenAddresses string) error {
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
		return formatSSHExecError(fmt.Errorf("find postgres config files: %w", err), out)
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
	listenCmd := "sudo sh -c " + shellQuote(listenScript)
	if out, err = sshClient.Run(listenCmd); err != nil {
		return formatSSHExecError(fmt.Errorf("configure listen_addresses: %w", err), out)
	}

	passwordEncScript := fmt.Sprintf(
		`if grep -Eq "^[#[:space:]]*password_encryption[[:space:]]*=" %s; then sed -i "s/^[#[:space:]]*password_encryption[[:space:]]*=.*/password_encryption = %s/" %s; else printf "\npassword_encryption = %s\n" >> %s; fi`,
		shellQuote(postgresqlConfPath),
		shellQuote("scram-sha-256"),
		shellQuote(postgresqlConfPath),
		shellQuote("scram-sha-256"),
		shellQuote(postgresqlConfPath),
	)
	passwordEncCmd := "sudo sh -c " + shellQuote(passwordEncScript)
	if out, err = sshClient.Run(passwordEncCmd); err != nil {
		return formatSSHExecError(fmt.Errorf("configure password_encryption: %w", err), out)
	}

	hbaRule := fmt.Sprintf("host all all %s %s", hbaCIDR, authMethod)
	hbaScript := fmt.Sprintf(
		`grep -Fqx %s %s || printf "\n%s\n" >> %s`,
		shellQuote(hbaRule),
		shellQuote(pgHbaConfPath),
		hbaRule,
		shellQuote(pgHbaConfPath),
	)
	hbaCmd := "sudo sh -c " + shellQuote(hbaScript)
	if out, err = sshClient.Run(hbaCmd); err != nil {
		return formatSSHExecError(fmt.Errorf("update pg_hba.conf: %w", err), out)
	}

	if err := restartRemotePostgresService(sshClient, postgresqlConfPath); err != nil {
		return err
	}

	return nil
}

func restartRemotePostgresService(sshClient *goph.Client, postgresqlConfPath string) error {
	dataDir := filepath.ToSlash(filepath.Dir(postgresqlConfPath))
	versionCandidate := ""
	parts := strings.Split(dataDir, "/")
	for i, part := range parts {
		if part == "pgsql" && i+1 < len(parts) && parts[i+1] != "" {
			versionCandidate = parts[i+1]
			break
		}
	}

	candidates := []string{
		"postgresql",
		"postgresql.service",
	}
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

	restartCmd := "sh -c " + shellQuote(restartScript)
	out, err := sshClient.Run(restartCmd)
	if err != nil {
		return formatSSHExecError(fmt.Errorf("restart postgresql service: %w", err), out)
	}

	return nil
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

var newAppDBCmd = &cobra.Command{
	Use:   "new-appdb",
	Short: "Create a new PostgreSQL database and service user for an application",
	Run: func(cmd *cobra.Command, args []string) {
		dbname := viper.GetString("db_name")
		username := viper.GetString("db_user")
		password := viper.GetString("db_password")
		createDB := viper.GetBool("create_db")
		dropFirst := viper.GetBool("drop_first")
		pgHostname := viper.GetString("postgres_host")
		pgPort := viper.GetInt("postgres_port")
		pgUser := viper.GetString("postgres_user")
		pgDb := viper.GetString("postgres_conn_db")
		pgPassword := viper.GetString("postgres_password")
		connectSSH := viper.GetBool("connect_ssh")
		sshHost := viper.GetString("ssh_host")
		sshUser := viper.GetString("ssh_user")
		sshKey := expandPath(viper.GetString("ssh_key"))
		sshPassphrase := viper.GetString("ssh_passphrase")
		useSshAgent := viper.GetBool("ssh_agent")
		sshPort := viper.GetInt("ssh_port")
		gooseyPath := viper.GetString("goosey_path")
		gooseyRunRemote := viper.GetBool("goosey_run_remote")
		gooseyBuildRemote := viper.GetBool("goosey_build_remote")
		gooseyRemoteGOOS := viper.GetString("goosey_remote_goos")
		gooseyRemoteGOARCH := viper.GetString("goosey_remote_goarch")
		setupRemotePostgres := viper.GetBool("setup_remote_postgres")
		remotePostgresCIDR := viper.GetString("remote_postgres_hba_cidr")
		remotePostgresAuthMethod := viper.GetString("remote_postgres_auth_method")
		remotePostgresListenAddresses := viper.GetString("remote_postgres_listen_addresses")

		if dropFirst {
			createDB = true
		}

		if sshKey == "" {
			sshKey = defaultSSHKeyPath()
		}

		// SSH-based execution path
		if connectSSH {
			if sshHost == "" {
				slog.Error("SSH host is required when using --connect-ssh")
				os.Exit(1)
			}
			if sshUser == "" {
				sshUser = currentUserName()
			}

			// Initialize SSH client
			sshClient, err := ssh.InitializeSshClient(sshHost, sshUser, sshKey, sshPassphrase, useSshAgent, uint(sshPort))
			if err != nil {
				slog.Error(
					"Failed to initialize SSH client",
					"host", sshHost,
					"user", sshUser,
					"port", sshPort,
					"ssh_key", sshKey,
					"use_ssh_agent", useSshAgent,
					"error", err.Error(),
				)
				os.Exit(1)
			}
			defer sshClient.Close()

			if setupRemotePostgres {
				slog.Info(
					"Configuring PostgreSQL for remote connections over SSH",
					"listen_addresses", remotePostgresListenAddresses,
					"hba_cidr", remotePostgresCIDR,
					"auth_method", remotePostgresAuthMethod,
				)
				if err := setupRemotePostgresAccess(sshClient, remotePostgresCIDR, remotePostgresAuthMethod, remotePostgresListenAddresses); err != nil {
					slog.Error("Failed configuring remote PostgreSQL access", "error", err.Error())
					os.Exit(1)
				}
			}

			dbname, username, password = promptForMissingAppConfig(cmd, dbname, username, password)

			checkStmt := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = %s)", pq.QuoteLiteral(dbname))
			dbExists, err := execSQLViaSshBool(sshClient, pgUser, "postgres", checkStmt)
			if err != nil {
				slog.Error("Failed to check if database exists via SSH", "error", err.Error())
				os.Exit(1)
			}

			if createDB && dropFirst {
				if err := dropAndRecreateDatabaseViaSSH(sshClient, pgUser, dbname); err != nil {
					slog.Error("Failed to drop and recreate database via SSH", "error", err.Error())
					os.Exit(1)
				}
				slog.Info("Database dropped and recreated", "DbName", dbname)
				dbExists = true
			} else if createDB {
				if dbExists {
					slog.Info("Database already exists. Skipping creation", "DbName", dbname)
				} else {
					createStmt := fmt.Sprintf(
						`CREATE DATABASE %s WITH OWNER = postgres ENCODING = %s TEMPLATE = template0;`,
						pq.QuoteIdentifier(dbname),
						pq.QuoteLiteral("UTF8"),
					)
					if err := execSQLViaSsh(sshClient, pgUser, "postgres", createStmt); err != nil {
						slog.Error("Failed to create database via SSH", "error", err.Error())
						os.Exit(1)
					}
					slog.Info("Database created", "DbName", dbname)
					dbExists = true
				}
			}

			// Create or alter user if SSH-based
			userCheckStmt := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = %s)", pq.QuoteLiteral(username))
			userExists, err := execSQLViaSshBool(sshClient, pgUser, "postgres", userCheckStmt)
			if err != nil {
				slog.Error("Failed to check if user exists via SSH", "error", err.Error())
				os.Exit(1)
			}
			if userExists {
				slog.Info("User already exists. Altering password", "username", username)
				alterPwStmt := fmt.Sprintf(`ALTER USER %s WITH PASSWORD %s;`, pq.QuoteIdentifier(username), pq.QuoteLiteral(password))
				if err := execSQLViaSsh(sshClient, pgUser, "postgres", alterPwStmt); err != nil {
					slog.Error("Failed to alter user password via SSH", "error", err.Error())
					os.Exit(2)
				}
			} else {
				createRoleStmt := fmt.Sprintf(`CREATE ROLE %s WITH LOGIN;`, pq.QuoteIdentifier(username))
				if err := execSQLViaSsh(sshClient, pgUser, "postgres", createRoleStmt); err != nil {
					slog.Error("Failed to create user via SSH", "error", err.Error())
					os.Exit(1)
				}
				alterPwStmt := fmt.Sprintf(`ALTER USER %s WITH PASSWORD %s;`, pq.QuoteIdentifier(username), pq.QuoteLiteral(password))
				if err := execSQLViaSsh(sshClient, pgUser, "postgres", alterPwStmt); err != nil {
					slog.Error("Failed to alter user via SSH", "error", err.Error())
					os.Exit(1)
				}
				slog.Info("User created", "username", username)
			}

			if !dbExists {
				if promptYesNo(fmt.Sprintf("Database %q does not exist. Create it now?", dbname), true) {
					createStmt := fmt.Sprintf(
						`CREATE DATABASE %s WITH OWNER = postgres ENCODING = %s TEMPLATE = template0;`,
						pq.QuoteIdentifier(dbname),
						pq.QuoteLiteral("UTF8"),
					)
					if err := execSQLViaSsh(sshClient, pgUser, "postgres", createStmt); err != nil {
						slog.Error("Failed to create database via SSH", "error", err.Error())
						os.Exit(1)
					}
					slog.Info("Database created", "DbName", dbname)
					dbExists = true
				} else {
					slog.Error("Target database does not exist", "DbName", dbname)
					os.Exit(1)
				}
			}

			// Grant database privileges
			grantDbStmt := fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE %s TO %s;`, pq.QuoteIdentifier(dbname), pq.QuoteIdentifier(username))
			if err := execSQLViaSsh(sshClient, pgUser, "postgres", grantDbStmt); err != nil {
				slog.Error("Failed to grant database privileges via SSH", "error", err.Error())
				os.Exit(1)
			}

			// Execute schema-level statements
			schemaStatements := []string{
				fmt.Sprintf(`GRANT ALL ON SCHEMA public TO %s;`, pq.QuoteIdentifier(username)),
				fmt.Sprintf(`ALTER SCHEMA public OWNER TO %s;`, pq.QuoteIdentifier(username)),
			}

			for _, stmt := range schemaStatements {
				slog.Info("Executing SQL via SSH", "Query", stmt)
				if err := execSQLViaSsh(sshClient, pgUser, dbname, stmt); err != nil {
					slog.Error("Failed executing statement via SSH", slog.String("Query", stmt), slog.String("error", err.Error()))
					os.Exit(1)
				}
			}

			if gooseyPath != "" {
				dbURL := buildPostgresURL(sshHost, pgPort, dbname, username, password)
				slog.Info("Running goosey migration", "path", expandPath(gooseyPath), "database", dbname, "host", sshHost, "port", pgPort, "user", username, "remote", gooseyRunRemote)
				var err error
				if gooseyRunRemote {
					gooseyBinaryPath, cleanup, buildErr := maybeBuildRemoteGooseyBinary(gooseyPath, gooseyBuildRemote, gooseyRemoteGOOS, gooseyRemoteGOARCH)
					if buildErr != nil {
						slog.Error("Failed preparing remote goosey binary", "error", buildErr.Error())
						os.Exit(1)
					}
					defer cleanup()
					err = runGooseyBinaryRemote(sshClient, gooseyBinaryPath, dbURL)
				} else {
					err = runGooseyBinary(gooseyPath, dbURL)
				}
				if err != nil {
					slog.Error("Failed running goosey migration", "error", err.Error())
					os.Exit(1)
				}
			}

			slog.Info("Privileges granted to app user on database via SSH", "Username", username, "DbName", dbname)
			return
		}

		// Direct connection path (original logic)
		connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", pgHostname, pgPort, pgUser, pgPassword, pgDb)
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			slog.Error("Failed to connect to PostgreSQL", "error", err.Error())
			os.Exit(1)
		}
		defer db.Close()

		dbname, username, password = promptForMissingAppConfig(cmd, dbname, username, password)

		var dbExists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbname).Scan(&dbExists)
		if err != nil {
			slog.Error("Failed to check if database exists", "error", err.Error())
			os.Exit(1)
		}

		if createDB && dropFirst {
			err = dropAndRecreateDatabaseDirect(db, dbname)
			if err != nil {
				slog.Error("Failed to drop and recreate database", "error", err.Error())
				os.Exit(1)
			}
			slog.Info("Database dropped and recreated", "DbName", dbname)
			dbExists = true
		} else if createDB {
			if dbExists {
				slog.Info("Database already exists. Skipping creation", "DbName", dbname)
			} else {
				_, err = db.Exec(fmt.Sprintf(
					`CREATE DATABASE %s WITH OWNER = postgres ENCODING = %s TEMPLATE = template0;`,
					pq.QuoteIdentifier(dbname),
					pq.QuoteLiteral("UTF8"),
				))
				if err != nil {
					slog.Error("Failed to create database", "error", err.Error())
				}
				slog.Info("Database created", "DbName", dbname)
				dbExists = true
			}
		}

		// Create user if it doesn't exist
		var userExists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", username).Scan(&userExists)
		if err != nil {
			slog.Error("Failed to check if user exists", "error", err.Error())
			os.Exit(1)
		}

		if userExists {
			slog.Info("User already exists. Altering password", "username", username)
			_, err = db.Exec(fmt.Sprintf(`ALTER USER %s WITH PASSWORD %s;`, pq.QuoteIdentifier(username), pq.QuoteLiteral(password)))
			if err != nil {
				slog.Error("Failed to alter user password", "error", err.Error())
				os.Exit(2)
			}
		} else {
			crtQry := fmt.Sprintf(`CREATE ROLE %s WITH LOGIN;`, pq.QuoteIdentifier(username))
			altrPwQry := fmt.Sprintf(`ALTER USER %s WITH PASSWORD %s;`, pq.QuoteIdentifier(username), pq.QuoteLiteral(password))
			_, err = db.Exec(crtQry)
			if err != nil {
				slog.Error("Failed to create user", "error", err.Error())
				os.Exit(1)
			}
			_, err = db.Exec(altrPwQry)
			if err != nil {
				slog.Error("Failed to alter user", "error", err.Error())
				os.Exit(1)
			}
			slog.Info("User created", "username", username)
		}

		if !dbExists {
			if promptYesNo(fmt.Sprintf("Database %q does not exist. Create it now?", dbname), true) {
				_, err = db.Exec(fmt.Sprintf(
					`CREATE DATABASE %s WITH OWNER = postgres ENCODING = %s TEMPLATE = template0;`,
					pq.QuoteIdentifier(dbname),
					pq.QuoteLiteral("UTF8"),
				))
				if err != nil {
					slog.Error("Failed to create database", "error", err.Error())
					os.Exit(1)
				}
				slog.Info("Database created", "DbName", dbname)
				dbExists = true
			} else {
				slog.Error("Target database does not exist", "DbName", dbname)
				os.Exit(1)
			}
		}

		// Grant CONNECT on database (idempotent)
		_, err = db.Exec(fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE %s TO %s;`, pq.QuoteIdentifier(dbname), pq.QuoteIdentifier(username)))
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			slog.Error("Failed to grant CONNECT", "error", err.Error())
			os.Exit(1)
		}

		// Connect to target database
		appDBConn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", pgHostname, pgPort, pgUser, pgPassword, dbname)
		appdb, err := sql.Open("postgres", appDBConn)
		if err != nil {
			slog.Error("Failed to connect to target database", "error", err.Error())
			os.Exit(1)
		}
		defer appdb.Close()

		sqlStatements := []string{
			// Need to revisit exact required permissions, but this works for development purposes...at least it's better that just creating a super user.
			fmt.Sprintf(`GRANT ALL ON SCHEMA public TO %s;`, pq.QuoteIdentifier(username)),
			fmt.Sprintf(`ALTER SCHEMA public OWNER TO %s;`, pq.QuoteIdentifier(username)),
			// Grant permissions to the schema
			//fmt.Sprintf(`GRANT USAGE ON SCHEMA public TO %s;`, username),
			//fmt.Sprintf(`GRANT CREATE ON SCHEMA public TO %s;`, username),
			//fmt.Sprintf(`GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO %s;`, username),
			// Grant access to all current sequences
			//fmt.Sprintf(`GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO %s;`, username),
			//fmt.Sprintf(`GRANT ALL ON SCHEMA public TO %s;`, username),
			// Alter default privileges for future tables and sequences
			//fmt.Sprintf(`ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %s;`, username),
			//fmt.Sprintf(`ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO %s;`, username),
			// Optional: Ensure user has ownership of schema (use with caution)

		}

		pgStatements := []string{
			// Grant access to all current tables
			fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE %s TO %s;`, pq.QuoteIdentifier(dbname), pq.QuoteIdentifier(username)),
		}

		for _, stmt := range sqlStatements {
			slog.Info("Executing SQL", "Query", stmt)
			if _, err := appdb.Exec(stmt); err != nil {
				slog.Error("Failed executing statement", slog.String("Query", stmt), slog.String("error", err.Error()))
				os.Exit(1)
			}
		}

		for _, stmt := range pgStatements {
			slog.Info("Executing SQL", "Query", stmt)
			if _, err := db.Exec(stmt); err != nil {
				slog.Error("Failed executing statement", slog.String("Query", stmt), slog.String("error", err.Error()))
				os.Exit(1)
			}
		}

		if gooseyPath != "" {
			dbURL := buildPostgresURL(pgHostname, pgPort, dbname, username, password)
			slog.Info("Running goosey migration", "path", expandPath(gooseyPath), "database", dbname, "host", pgHostname, "port", pgPort, "user", username)
			if err := runGooseyBinary(gooseyPath, dbURL); err != nil {
				slog.Error("Failed running goosey migration", "error", err.Error())
				os.Exit(1)
			}
		}

		slog.Info("Privileges granted to app user on database", "Username", username, "DbName", dbname)
	},
}

func init() {
	newAppDBCmd.Flags().String("db-name", "", "Name of the database to create/configure")
	newAppDBCmd.Flags().String("db-user", "", "Service user name to create")
	newAppDBCmd.Flags().String("db-password", "", "Password for the service user")
	newAppDBCmd.Flags().Bool("create-db", false, "Create the database if it doesn't exist")
	newAppDBCmd.Flags().Bool("drop-first", false, "Drop and recreate the application database before applying grants")
	newAppDBCmd.Flags().String("postgres-password", "", "PostgreSQL superuser password")
	newAppDBCmd.Flags().String("postgres-user", "postgres", "PostgreSQL admin username")
	newAppDBCmd.Flags().String("postgres-hostname", "localhost", "PostgreSQL server hostname")
	newAppDBCmd.Flags().String("postgres-conn-db", "postgres", "Initial connection database")
	newAppDBCmd.Flags().Int("postgres-port", 5432, "PostgreSQL port")
	newAppDBCmd.Flags().Bool("connect-ssh", false, "Connect to PostgreSQL via SSH instead of direct connection")
	newAppDBCmd.Flags().String("goosey-path", "", "Path to a goosey migration binary to run after database/user setup")
	newAppDBCmd.Flags().Bool("goosey-run-remote", false, "Upload the goosey binary to the remote host and run it there instead of locally")
	newAppDBCmd.Flags().Bool("goosey-build-remote", false, "Build a remote-compatible goosey binary from the goosey source directory before uploading it")
	newAppDBCmd.Flags().String("goosey-remote-goos", "linux", "GOOS to use when building a remote goosey binary")
	newAppDBCmd.Flags().String("goosey-remote-goarch", "amd64", "GOARCH to use when building a remote goosey binary")
	newAppDBCmd.Flags().Bool("setup-remote-postgres", false, "Via SSH, configure postgresql.conf and pg_hba.conf to allow remote PostgreSQL connections")
	newAppDBCmd.Flags().String("remote-postgres-hba-cidr", "0.0.0.0/0", "CIDR to add to pg_hba.conf when enabling remote PostgreSQL access")
	newAppDBCmd.Flags().String("remote-postgres-auth-method", "scram-sha-256", "Authentication method to add to pg_hba.conf when enabling remote PostgreSQL access")
	newAppDBCmd.Flags().String("remote-postgres-listen-addresses", "*", "listen_addresses value to set in postgresql.conf when enabling remote PostgreSQL access")
	newAppDBCmd.Flags().String("ssh-host", "", "SSH host to connect to (required when using --connect-ssh)")
	newAppDBCmd.Flags().String("ssh-user", "", "SSH username (defaults to current user if not specified)")
	newAppDBCmd.Flags().String("ssh-key", "", "Path to SSH private key")
	newAppDBCmd.Flags().String("ssh-passphrase", "", "SSH key passphrase")
	newAppDBCmd.Flags().Bool("ssh-agent", false, "Use SSH agent for authentication")
	newAppDBCmd.Flags().Int("ssh-port", 22, "SSH port")

	viper.BindPFlag("db_name", newAppDBCmd.Flags().Lookup("db-name"))
	viper.BindPFlag("db_user", newAppDBCmd.Flags().Lookup("db-user"))
	viper.BindPFlag("db_password", newAppDBCmd.Flags().Lookup("db-password"))
	viper.BindPFlag("create_db", newAppDBCmd.Flags().Lookup("create-db"))
	viper.BindPFlag("drop_first", newAppDBCmd.Flags().Lookup("drop-first"))
	viper.BindPFlag("postgres_password", newAppDBCmd.Flags().Lookup("postgres-password"))
	viper.BindPFlag("postgres_user", newAppDBCmd.Flags().Lookup("postgres-user"))
	viper.BindPFlag("postgres_host", newAppDBCmd.Flags().Lookup("postgres-hostname"))
	viper.BindPFlag("postgres_port", newAppDBCmd.Flags().Lookup("postgres-port"))
	viper.BindPFlag("postgres_conn_db", newAppDBCmd.Flags().Lookup("postgres-conn-db"))
	viper.BindPFlag("connect_ssh", newAppDBCmd.Flags().Lookup("connect-ssh"))
	viper.BindPFlag("goosey_path", newAppDBCmd.Flags().Lookup("goosey-path"))
	viper.BindPFlag("goosey_run_remote", newAppDBCmd.Flags().Lookup("goosey-run-remote"))
	viper.BindPFlag("goosey_build_remote", newAppDBCmd.Flags().Lookup("goosey-build-remote"))
	viper.BindPFlag("goosey_remote_goos", newAppDBCmd.Flags().Lookup("goosey-remote-goos"))
	viper.BindPFlag("goosey_remote_goarch", newAppDBCmd.Flags().Lookup("goosey-remote-goarch"))
	viper.BindPFlag("setup_remote_postgres", newAppDBCmd.Flags().Lookup("setup-remote-postgres"))
	viper.BindPFlag("remote_postgres_hba_cidr", newAppDBCmd.Flags().Lookup("remote-postgres-hba-cidr"))
	viper.BindPFlag("remote_postgres_auth_method", newAppDBCmd.Flags().Lookup("remote-postgres-auth-method"))
	viper.BindPFlag("remote_postgres_listen_addresses", newAppDBCmd.Flags().Lookup("remote-postgres-listen-addresses"))
	viper.BindPFlag("ssh_host", newAppDBCmd.Flags().Lookup("ssh-host"))
	viper.BindPFlag("ssh_user", newAppDBCmd.Flags().Lookup("ssh-user"))
	viper.BindPFlag("ssh_key", newAppDBCmd.Flags().Lookup("ssh-key"))
	viper.BindPFlag("ssh_passphrase", newAppDBCmd.Flags().Lookup("ssh-passphrase"))
	viper.BindPFlag("ssh_agent", newAppDBCmd.Flags().Lookup("ssh-agent"))
	viper.BindPFlag("ssh_port", newAppDBCmd.Flags().Lookup("ssh-port"))

	viper.AutomaticEnv()

	viper.BindEnv("postgres_host", "PG_HOST")
	viper.BindEnv("postgres_port", "PG_PORT")
	viper.BindEnv("postgres_user", "PG_USER")
	viper.BindEnv("postgres_password", "PG_PASSWORD")
	viper.BindEnv("postgres_conn_db", "PG_DB")

	databaseCmd.AddCommand(newAppDBCmd)
}

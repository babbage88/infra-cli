package cmd

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/babbage88/goph/v2"
	"github.com/babbage88/infra-cli/ssh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/lib/pq"
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

var newAppDBCmd = &cobra.Command{
	Use:   "new-appdb",
	Short: "Create a new PostgreSQL database and service user for an application",
	Run: func(cmd *cobra.Command, args []string) {
		dbname := viper.GetString("db_name")
		username := viper.GetString("db_user")
		password := viper.GetString("db_password")
		createDB := viper.GetBool("create_db")
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
				slog.Error("Failed to initialize SSH client", "error", err.Error())
				os.Exit(1)
			}
			defer sshClient.Close()

			checkStmt := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = %s)", pq.QuoteLiteral(dbname))
			dbExists, err := execSQLViaSshBool(sshClient, pgUser, "postgres", checkStmt)
			if err != nil {
				slog.Error("Failed to check if database exists via SSH", "error", err.Error())
				os.Exit(1)
			}

			if createDB {
				if dbExists {
					slog.Info("Database already exists. Skipping creation", "DbName", dbname)
				} else {
					// Create database
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
				slog.Error("Target database does not exist. Re-run with --create-db or specify an existing --db-name", "DbName", dbname)
				os.Exit(1)
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

		if createDB {
			var exists bool
			err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbname).Scan(&exists)
			if err != nil {
				slog.Error("Failed to check if database exists", "error", err.Error())
				os.Exit(1)
			}

			if exists {
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

		slog.Info("Privileges granted to app user on database", "Username", username, "DbName", dbname)
	},
}

func init() {
	newAppDBCmd.Flags().String("db-name", "smbplusplus", "Name of the database to create/configure")
	newAppDBCmd.Flags().String("db-user", "smbp_user", "Service user name to create")
	newAppDBCmd.Flags().String("db-password", "changeMe123", "Password for the service user")
	newAppDBCmd.Flags().Bool("create-db", false, "Create the database if it doesn't exist")
	newAppDBCmd.Flags().String("postgres-password", "", "PostgreSQL superuser password")
	newAppDBCmd.Flags().String("postgres-user", "postgres", "PostgreSQL admin username")
	newAppDBCmd.Flags().String("postgres-hostname", "localhost", "PostgreSQL server hostname")
	newAppDBCmd.Flags().String("postgres-conn-db", "postgres", "Initial connection database")
	newAppDBCmd.Flags().Int("postgres-port", 5432, "PostgreSQL port")
	newAppDBCmd.Flags().Bool("connect-ssh", false, "Connect to PostgreSQL via SSH instead of direct connection")
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
	viper.BindPFlag("postgres_password", newAppDBCmd.Flags().Lookup("postgres-password"))
	viper.BindPFlag("postgres_user", newAppDBCmd.Flags().Lookup("postgres-user"))
	viper.BindPFlag("postgres_host", newAppDBCmd.Flags().Lookup("postgres-hostname"))
	viper.BindPFlag("postgres_port", newAppDBCmd.Flags().Lookup("postgres-port"))
	viper.BindPFlag("postgres_conn_db", newAppDBCmd.Flags().Lookup("postgres-conn-db"))
	viper.BindPFlag("connect_ssh", newAppDBCmd.Flags().Lookup("connect-ssh"))
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

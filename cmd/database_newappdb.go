package cmd

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	_ "github.com/lib/pq"
)

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
				slog.Error("Database %s already exists. Skipping creation", slog.String("DbName", dbname), slog.String("error", err.Error()))
			} else {
				_, err = db.Exec(fmt.Sprintf(`CREATE DATABASE %s WITH OWNER = postgres ENCODING = 'UTF8' TEMPLATE = template0;`, dbname))
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
			_, err = db.Exec(fmt.Sprintf(`ALTER USER %s WITH PASSWORD '%s';`, username, password))
			if err != nil {
				slog.Error("Failed to alter user password", "error", err.Error())
				os.Exit(2)
			}
		} else {
			crtQry := fmt.Sprintf(`CREATE ROLE %s WITH LOGIN;`, username)
			altrPwQry := fmt.Sprintf(`ALTER USER %s WITH PASSWORD '%s';`, username, password)
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
		_, err = db.Exec(fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE %s TO %s;`, dbname, username))
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
			fmt.Sprintf(`GRANT ALL ON SCHEMA public TO %s;`, username),
			fmt.Sprintf(`ALTER SCHEMA public OWNER TO %s;`, username),
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
			fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE %s TO %s;`, dbname, username),
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

	viper.BindPFlag("db_name", newAppDBCmd.Flags().Lookup("db-name"))
	viper.BindPFlag("db_user", newAppDBCmd.Flags().Lookup("db-user"))
	viper.BindPFlag("db_password", newAppDBCmd.Flags().Lookup("db-password"))
	viper.BindPFlag("create_db", newAppDBCmd.Flags().Lookup("create-db"))
	viper.BindPFlag("postgres_password", newAppDBCmd.Flags().Lookup("postgres-password"))
	viper.BindPFlag("postgres_user", newAppDBCmd.Flags().Lookup("postgres-user"))
	viper.BindPFlag("postgres_host", newAppDBCmd.Flags().Lookup("postgres-hostname"))
	viper.BindPFlag("postgres_port", newAppDBCmd.Flags().Lookup("postgres-port"))
	viper.BindPFlag("postgres_conn_db", newAppDBCmd.Flags().Lookup("postgres-conn-db"))

	viper.AutomaticEnv()

	viper.BindEnv("postgres_host", "PG_HOST")
	viper.BindEnv("postgres_port", "PG_PORT")
	viper.BindEnv("postgres_user", "PG_USER")
	viper.BindEnv("postgres_password", "PG_PASSWORD")
	viper.BindEnv("postgres_conn_db", "PG_DB")

	databaseCmd.AddCommand(newAppDBCmd)
}

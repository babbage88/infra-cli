package dbhelper

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	_ "github.com/lib/pq"
)

type PgSetupScriptsRequest struct {
	DbHostName        string `json:"dbHostname"`
	SuperuserUsername string `json:"superuserUsername"`
	SuperuserPassword string `json:"superuserPassword"`
	DatabaseName      string `json:"databaseName"`
	ServiceUsername   string `json:"serviceUsername"`
	ServicePassword   string `json:"servicePassword"`
	DbPort            int32  `json:"dbPort"`
}

func GenerateDbUserScriptsHandler() func(w http.ResponseWriter, r *http.Request) {
	return generateDbUserScriptsHandler
}

// Example Go Handler (with a simple HTTP route)
func generateDbUserScriptsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		// Preflight request
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusOK)
		return
	}
	// Extract the parameters from the request (e.g., JSON payload)
	var requestData PgSetupScriptsRequest

	// Parse the JSON request
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlScripts := GenerateSqlScript(requestData.DbHostName,
		requestData.SuperuserUsername,
		requestData.SuperuserPassword,
		requestData.ServiceUsername,
		requestData.ServicePassword,
		requestData.DatabaseName,
		requestData.DbPort)

	// Send the script back to the frontend
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sqlScripts)
}

type PgDevDbSetupScriptsResponse struct {
	ShellScript      string `json:"create_dev_db.sh"`
	PgCreateScript   string `json:"pg_create_db.sql"`
	AppDbSetupScript string `json:"pg_app_db.sql"`
}

func GenerateSqlScript(dbHostname, superUsername, superUserPassword, appUsername, appPass, appDbName string, dbPort int32) PgDevDbSetupScriptsResponse {
	var sqlScript strings.Builder
	var appSqlScript strings.Builder
	var shellScript strings.Builder
	pgDb := "postgres"

	pgSqlScriptName := "pg_create_db.sql"
	appDbScriptName := "pg_app_db.sql"

	shellScript.WriteString("#!/bin/sh\n")

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dbHostname, dbPort, superUsername, superUserPassword, pgDb)
	appDBConn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dbHostname, dbPort, superUsername, superUserPassword, appDbName)

	shellScript.WriteString(fmt.Sprintf("psql -Atx \"%s\" -f %s\n", connStr, pgSqlScriptName))
	shellScript.WriteString(fmt.Sprintf("psql -Atx \"%s\" -f %s\n", appDBConn, appDbScriptName))

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		slog.Warn("error connecting to database, assuming db and user do not exist for script generation", slog.String("error", err.Error()))
	}
	defer db.Close()

	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", appDbName).Scan(&exists)
	if err != nil {
		slog.Warn("error checking if db exists, assuming it does not exist", slog.String("error", err.Error()))
	}

	if !exists {
		sqlScript.WriteString("/* #### Creating new database #### */")
		sqlScript.WriteByte('\n')
		sqlScript.WriteString(fmt.Sprintf(`CREATE DATABASE %s WITH OWNER = postgres ENCODING = 'UTF8' TEMPLATE = template0;`, appDbName))
		sqlScript.WriteByte('\n')
	}

	var userExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", appUsername).Scan(&userExists)
	if err != nil {
		slog.Warn("error checking if user exists, assuming it does not", slog.String("error", err.Error()))
	}

	if userExists {
		sqlScript.WriteString(fmt.Sprintf(`ALTER USER %s WITH PASSWORD '%s';`, appUsername, appPass))
		sqlScript.WriteByte('\n')
	} else {
		sqlScript.WriteString(fmt.Sprintf(`CREATE ROLE %s WITH LOGIN;`, appUsername))
		sqlScript.WriteByte('\n')
		sqlScript.WriteString(fmt.Sprintf(`ALTER USER %s WITH PASSWORD '%s';`, appUsername, appPass))
		sqlScript.WriteByte('\n')
	}

	sqlScript.WriteString(fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE %s TO %s;`, appDbName, appUsername))
	sqlScript.WriteByte('\n')

	appdb, err := sql.Open("postgres", appDBConn)
	if err != nil {
		slog.Warn("failed to connect to target database", slog.String("error", err.Error()))
	}
	defer appdb.Close()

	appSqlScript.WriteString("/* ######### SQL Statements to execute while connected to the new application database ########## */\n")
	appSqlScript.WriteString(fmt.Sprintf(`GRANT ALL ON SCHEMA public TO %s;`, appUsername))
	appSqlScript.WriteByte('\n')
	appSqlScript.WriteString(fmt.Sprintf(`ALTER SCHEMA public OWNER TO %s;`, appUsername))
	appSqlScript.WriteByte('\n')

	sqlScript.WriteString("/* ######### SQL Statements to execute while connected to the default postgres database ########## */\n")
	sqlScript.WriteString(fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE %s TO %s;`, appDbName, appUsername))
	sqlScript.WriteByte('\n')

	return PgDevDbSetupScriptsResponse{
		ShellScript:      shellScript.String(),
		PgCreateScript:   sqlScript.String(),
		AppDbSetupScript: appSqlScript.String(),
	}
}

type PgDevDbExecStatement struct {
	QryCmd []string `json:"qryCmds"`
	DbName string   `json:"dbName"`
	DbUser string   `json:"dbUser"`
}

type PgCreateDevDbAndUserResponse struct {
	StatemensExecuted []PgDevDbExecStatement `json:"pdDevDbDeploymentStatements"`
	Errors            []error                `json:"deploymentErrors"`
}

// Generates two postgres sql scripts, one to run while connected to the postgres db and one connected to the new appDb after it has been created.
func generateSqlScriptAndExecute(dbHostname, superUsername, superUserPassword, appUsername, appPass, appDbName string, dbPort int16) string {
	var sqlScript strings.Builder
	var pgDb string = "postgres"
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dbHostname, dbPort, superUsername, superUserPassword, pgDb)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		slog.Error("error connecting to database", slog.String("error", err.Error()))
	}
	defer db.Close()

	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", appDbName).Scan(&exists)
	if err != nil {
		slog.Error("error checking if db exists to database", slog.String("error", err.Error()))
	}

	if exists {
		slog.Info("Database already exists. Skipping creation", "appDbName", appDbName)
	} else {

		crtDbQry := fmt.Sprintf(`CREATE DATABASE %s WITH OWNER = postgres ENCODING = 'UTF8' TEMPLATE = template0;`, appDbName)
		sqlScript.WriteString(crtDbQry)
		sqlScript.WriteByte('\n')

		_, err = db.Exec(crtDbQry)
		if err != nil {
			slog.Error("error checking if db exists to database", slog.String("error", err.Error()))
		}
		slog.Info("Database created", "New_Database", appDbName)
	}

	// Create user if it doesn't exist
	var userExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", appUsername).Scan(&userExists)
	if err != nil {
		slog.Error("Failed while checking if user exists", slog.String("error", err.Error()))
	}

	if userExists {
		slog.Info("User already exists. Altering password", "appUsername", appUsername)
		altrUsrQry := fmt.Sprintf(`ALTER USER %s WITH PASSWORD '%s';`, appUsername, appPass)
		sqlScript.WriteString(altrUsrQry)
		sqlScript.WriteByte('\n')
		_, err = db.Exec(altrUsrQry)
		if err != nil {
			slog.Error("Failed to alter appUsername password", slog.String("error", err.Error()))
		}
	} else {
		crtUsrQry := fmt.Sprintf(`CREATE ROLE %s WITH LOGIN;`, appUsername)
		sqlScript.WriteString(crtUsrQry)
		sqlScript.WriteByte('\n')

		altrPwQry := fmt.Sprintf(`ALTER USER %s WITH PASSWORD '%s';`, appUsername, appPass)
		sqlScript.WriteString(altrPwQry)
		sqlScript.WriteByte('\n')

		_, err = db.Exec(crtUsrQry)
		if err != nil {
			slog.Error("Failed to create user", "error", err.Error())
		}
		_, err = db.Exec(altrPwQry)
		if err != nil {
			slog.Error("Failed to alter user", "error", err.Error())
		}
		slog.Info("User created", "New User", appUsername)

	}

	// Grant CONNECT on database (idempotent)
	grantPrivsQry := fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE %s TO %s;`, appDbName, appUsername)
	sqlScript.WriteString(grantPrivsQry)
	sqlScript.WriteByte('\n')

	_, err = db.Exec(grantPrivsQry)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		slog.Error("Failed to grant CONNECT", "error", err.Error())
		os.Exit(1)
	}

	// Connect to target database
	appDBConn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dbHostname, dbPort, superUsername, superUserPassword, appDbName)
	appdb, err := sql.Open("postgres", appDBConn)
	if err != nil {
		slog.Error("Failed to connect to target database", "error", err.Error())
		os.Exit(1)
	}
	defer appdb.Close()

	sqlStatements := []string{
		// Need to revisit exact required permissions, but this works for development purposes...at least it's better that just creating a super user.
		fmt.Sprintf(`GRANT ALL ON SCHEMA public TO %s;`, appUsername),
		fmt.Sprintf(`ALTER SCHEMA public OWNER TO %s;`, appUsername),
	}

	pgStatements := []string{
		// Grant access to all current tables
		fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE %s TO %s;`, appDbName, appUsername),
	}

	sqlScript.WriteString("/* ######### SQL Statements to execute while connected to the new application database ########## */")
	sqlScript.WriteByte('\n')

	for _, stmt := range sqlStatements {
		sqlScript.WriteString(stmt)
		sqlScript.WriteByte('\n')

		slog.Info("Executing SQL", "Query", stmt)
		if _, err := appdb.Exec(stmt); err != nil {
			slog.Error("Failed executing statement", slog.String("Query", stmt), slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	sqlScript.WriteString("/* ######### SQL Statements to execute while connected to the default postgres database ########## */")
	sqlScript.WriteByte('\n')

	for _, stmt := range pgStatements {
		sqlScript.WriteString(stmt)
		sqlScript.WriteByte('\n')
		slog.Info("Executing SQL", "Query", stmt)
		if _, err := db.Exec(stmt); err != nil {
			slog.Error("Failed executing statement", slog.String("Query", stmt), slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	return sqlScript.String()
}

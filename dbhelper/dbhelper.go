package dbhelper

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	_ "github.com/lib/pq"
)

// Example Go Handler (with a simple HTTP route)
func generateDbUserScript(w http.ResponseWriter, r *http.Request) {
	// Extract the parameters from the request (e.g., JSON payload)
	var requestData struct {
		SuperuserUsername string `json:"superuserUsername"`
		SuperuserPassword string `json:"superuserPassword"`
		DatabaseName      string `json:"databaseName"`
		ServiceUsername   string `json:"serviceUsername"`
		ServicePassword   string `json:"servicePassword"`
	}

	// Parse the JSON request
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create SQL script (mock example)
	sqlScript := fmt.Sprintf(`
    CREATE DATABASE %s;

    CREATE USER %s WITH ENCRYPTED PASSWORD '%s';

    GRANT ALL PRIVILEGES ON DATABASE %s TO %s;
  `, requestData.DatabaseName, requestData.ServiceUsername, requestData.ServicePassword, requestData.DatabaseName, requestData.ServiceUsername)

	// Send the script back to the frontend
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"script": sqlScript})
}

func generateSqlScript(dbHostname, superUsername, superUserPassword, appUsername, appPass, appDbName string, dbPort int16) string {
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
		fmt.Printf("Database %s already exists. Skipping creation.\n", appDbName)
	} else {

		crtDbQry := fmt.Sprintf(`CREATE DATABASE %s WITH OWNER = postgres ENCODING = 'UTF8' TEMPLATE = template0;`, appDbName)
		sqlScript.WriteString(crtDbQry)
		_, err = db.Exec(crtDbQry)
		if err != nil {
			slog.Error("error checking if db exists to database", slog.String("error", err.Error()))
		}
		fmt.Printf("Database %s created.\n", appDbName)
	}

	// Create user if it doesn't exist
	var userExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", appUsername).Scan(&userExists)
	if err != nil {
		slog.Error("Failed while checking if user exists", slog.String("error", err.Error()))
	}

	if userExists {
		fmt.Printf("User %s already exists. Altering password.\n", appUsername)
		altrUsrQry := fmt.Sprintf(`ALTER USER %s WITH PASSWORD '%s';`, appUsername, appPass)
		_, err = db.Exec(altrUsrQry)
		if err != nil {
			slog.Error("Failed to alter appUsername password", slog.String("error", err.Error()))
		}
	} else {
		crtUsrQry := fmt.Sprintf(`CREATE ROLE %s WITH LOGIN;`, appUsername)
		sqlScript.WriteString(crtUsrQry)
	}
	return sqlScript.String()
}

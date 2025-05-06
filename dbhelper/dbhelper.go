package dbhelper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

func generateSqlScript(superUsername, superUserPassword, appUsername, appPass, appDbName string) string {
	var sqlScript strings.Builder

	return sqlScript.String()
}

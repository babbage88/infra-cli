package cmd

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/babbage88/infra-cli/dbhelper"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/spf13/cobra"
)

var dbHelperApiCmd = &cobra.Command{
	Use:   "db-helper-api",
	Short: "Start Db Helper API databases",
	Run: func(cmd *cobra.Command, args []string) {
		listenPort, _ := cmd.Flags().GetInt32("listen-address")
		startApi, _ := cmd.Flags().GetBool("start-api")
		listenAddr := fmt.Sprintf(":%d", listenPort)
		if startApi {
			startApiServer(&listenAddr)
		}

	},
}

func init() {
	databaseCmd.AddCommand(dbHelperApiCmd)
	dbHelperApiCmd.Flags().Bool("start-api", false, "Flag to start api")
	dbHelperApiCmd.Flags().Int32("listen-address", 8181, "Port to listen on.")
}

func startApiServer(listenAddr *string) error {
	mux := http.NewServeMux()
	slog.Info("Starting Db Helper UI API Server", slog.String("ListedAddr", *listenAddr))
	mux.Handle("/api/generate-pg-setup-scripts", http.HandlerFunc(dbhelper.GenerateDbUserScriptsHandler()))
	//mux.Handle("/api/download-pg-scripts", http.HandlerFunc(dbhelper.DownloadDbUserScriptsHandler()))
	mux.HandleFunc("/api/download-pg-scripts", dbhelper.DownloadDbUserScriptsHandler())
	mux.Handle("/metrics", promhttp.Handler())
	return http.ListenAndServe(*listenAddr, handleCORSOptions(mux))

}

func handleCORSOptions(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

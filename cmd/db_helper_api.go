package cmd

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

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
	mux.Handle("/generate-pg-setup-scripts", http.HandlerFunc(dbhelper.GenerateDbUserScriptsHandler()))
	//mux.Handle("/api/download-pg-scripts", http.HandlerFunc(dbhelper.DownloadDbUserScriptsHandler()))
	mux.HandleFunc("/download-pg-scripts", dbhelper.DownloadDbUserScriptsHandler())
	mux.Handle("/metrics", promhttp.Handler())
	return http.ListenAndServe(*listenAddr, requestLoggingMiddleware(handleCORSOptions(mux)))

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

// statusRecorder wraps http.ResponseWriter to capture the status code
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// requestLoggingMiddleware logs request paths and warns on 404s
func requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(recorder, r)

		duration := time.Since(start)

		if recorder.status == http.StatusNotFound {
			slog.Warn("no route matched",
				slog.String("path", r.URL.Path),
				slog.String("method", r.Method),
				slog.Int("status", recorder.status),
				slog.Any("duration", duration),
			)
		} else {
			slog.Info("handled request",
				slog.String("path", r.URL.Path),
				slog.String("method", r.Method),
				slog.Int("status", recorder.status),
				slog.Any("duration", duration),
			)
		}
	})
}

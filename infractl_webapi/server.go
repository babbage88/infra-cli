package infractl_webapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	coredeploy "github.com/babbage88/infra-core/deployment"
)

type ServerOptions struct {
	DefaultSSH coredeploy.SSHOptions
}

type Server struct {
	mux        *http.ServeMux
	defaultSSH coredeploy.SSHOptions
}

func NewServer(opts ServerOptions) *Server {
	srv := &Server{
		mux:        http.NewServeMux(),
		defaultSSH: opts.DefaultSSH,
	}
	srv.routes()
	return srv
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, requestLoggingMiddleware(handleCORSOptions(s.mux)))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/v1/health", s.healthHandler)
	s.mux.HandleFunc("POST /api/v1/database/mariadb/install", s.installMariaDBHandler)
	s.mux.HandleFunc("POST /api/v1/database/valkey/install", s.installValkeyHandler)
	s.mux.HandleFunc("POST /api/v1/proxy/{name}/install", s.installProxyHandler)
	s.mux.HandleFunc("POST /api/v1/storage/s3/garage/node", s.deployGarageNodeHandler)
	s.mux.HandleFunc("POST /api/v1/storage/s3/garage/token", s.createGarageTokenHandler)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("failed to write JSON response", "error", err.Error())
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func handleCORSOptions(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(recorder, r)

		duration := time.Since(start)
		attrs := []any{
			slog.String("path", r.URL.Path),
			slog.String("method", r.Method),
			slog.Int("status", recorder.status),
			slog.Any("duration", duration),
		}
		if recorder.status == http.StatusNotFound {
			slog.Warn("no route matched", attrs...)
			return
		}
		slog.Info("handled request", attrs...)
	})
}

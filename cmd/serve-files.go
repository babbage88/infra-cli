package cmd

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	srvSrcPath string
	port       int
)

var serveCmd = &cobra.Command{
	Use:   "serve-files",
	Short: "Serve static files from a directory",
	Run: func(cmd *cobra.Command, args []string) {
		// Ensure path exists
		absPath, err := filepath.Abs(srvSrcPath)
		if err != nil {
			slog.Error("Error resolving path", slog.String("error", err.Error()))
			os.Exit(1)
		}

		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			slog.Error("Directory does not exist", "path", absPath)
			os.Exit(1)
		}

		fs := http.FileServer(http.Dir(absPath))
		http.Handle("/", fs)

		addr := fmt.Sprintf(":%d", port)
		slog.Info("Serving files", slog.String("path", absPath), slog.String("port", addr))

		if err := http.ListenAndServe(addr, nil); err != nil {
			slog.Error("Server failed", "error", err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVarP(&srvSrcPath, "path", "p", ".", "Path to serve static files from")
	serveCmd.Flags().IntVarP(&port, "port", "", 8080, "Port to run the HTTP server on")
}

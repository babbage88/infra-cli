package cmd

import (
	"fmt"
	"log/slog"

	webapi "github.com/babbage88/infra-cli/infractl_webapi"
	infraSSH "github.com/babbage88/infra-cli/ssh"
	coredeploy "github.com/babbage88/infra-core/deployment"
	"github.com/spf13/cobra"
)

var apiServeCmd = &cobra.Command{
	Use:   "api",
	Short: "Start the infractl HTTP API",
}

var apiServeStartCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve infractl commands over HTTP",
	RunE: func(cmd *cobra.Command, args []string) error {
		listenAddr, _ := cmd.Flags().GetString("listen-address")
		if listenAddr == "" {
			listenAddr = ":8181"
			slog.Warn("API listen address was empty, using default", "listen_addr", listenAddr)
		}

		defaultSSH := coredeploy.SSHOptions{
			Host:       rootViperCfg.GetString("ssh_remote_host"),
			User:       rootViperCfg.GetString("ssh_remote_user"),
			KeyPath:    infraSSH.ExpandPath(rootViperCfg.GetString("ssh_key")),
			Passphrase: rootViperCfg.GetString("ssh_passphrase"),
			UseAgent:   rootViperCfg.GetBool("ssh_use_agent"),
			Port:       rootViperCfg.GetUint("ssh_port"),
		}
		if defaultSSH.Port == 0 {
			defaultSSH.Port = 22
			slog.Debug("SSH port was unset, using default", "ssh_port", defaultSSH.Port)
		}
		if defaultSSH.User == "" {
			defaultSSH.User = infraSSH.CurrentUserName()
			slog.Debug("SSH user was unset, using current user", "ssh_user", defaultSSH.User)
		}

		logger := slog.With(
			slog.String("listen_addr", listenAddr),
			slog.String("ssh_host", defaultSSH.Host),
			slog.String("ssh_user", defaultSSH.User),
			slog.Uint64("ssh_port", uint64(defaultSSH.Port)),
			slog.String("ssh_key", defaultSSH.KeyPath),
			slog.Bool("ssh_use_agent", defaultSSH.UseAgent),
			slog.Bool("ssh_passphrase_configured", defaultSSH.Passphrase != ""),
		)
		logger.Info("Starting infractl API server")

		server := webapi.NewServer(webapi.ServerOptions{DefaultSSH: defaultSSH})
		if err := server.ListenAndServe(listenAddr); err != nil {
			logger.Error("Infractl API server stopped with error", "error", err.Error())
			return fmt.Errorf("serve infractl API: %w", err)
		}
		logger.Info("Infractl API server stopped")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(apiServeCmd)
	apiServeCmd.AddCommand(apiServeStartCmd)
	apiServeStartCmd.Flags().String("listen-address", ":8181", "Address the infractl API should listen on")
}

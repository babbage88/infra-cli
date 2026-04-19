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
		}
		if defaultSSH.User == "" {
			defaultSSH.User = infraSSH.CurrentUserName()
		}

		slog.Info("Starting infractl API server", slog.String("listen_addr", listenAddr))
		server := webapi.NewServer(webapi.ServerOptions{DefaultSSH: defaultSSH})
		if err := server.ListenAndServe(listenAddr); err != nil {
			return fmt.Errorf("serve infractl API: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(apiServeCmd)
	apiServeCmd.AddCommand(apiServeStartCmd)
	apiServeStartCmd.Flags().String("listen-address", ":8181", "Address the infractl API should listen on")
}

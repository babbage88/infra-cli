package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/babbage88/infra-cli/deployer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var valkeyNewViper *viper.Viper

var databaseValkeyNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Install and configure Valkey for remote access on a host over SSH",
	Run: func(cmd *cobra.Command, args []string) {
		sshOpts, err := resolveRootSSHOptions("", "")
		if err != nil {
			slog.Error("Failed to resolve SSH options", "error", err.Error())
			os.Exit(1)
		}

		valkeyUsername := valkeyNewViper.GetString("valkey_username")
		valkeyPassword := valkeyNewViper.GetString("valkey_password")
		valkeyBind := valkeyNewViper.GetString("valkey_bind")
		valkeyPort := valkeyNewViper.GetInt("valkey_port")
		valkeyACLFile := valkeyNewViper.GetString("valkey_acl_file")

		if valkeyPort <= 0 {
			slog.Error("Invalid Valkey port", "port", valkeyPort)
			os.Exit(1)
		}

		if !cmd.Flags().Changed("username") {
			valkeyUsername = promptInput("Valkey ACL username", valkeyUsername)
		}
		if !cmd.Flags().Changed("password") {
			valkeyPassword = promptPassword("Valkey ACL password", valkeyPassword)
		}

		installer, err := deployer.NewRemoteValkeyInstallerWithSsh(
			sshOpts.Host,
			sshOpts.User,
			sshOpts.KeyPath,
			sshOpts.Passphrase,
			sshOpts.UseAgent,
			sshOpts.Port,
		)
		if err != nil {
			slog.Error(
				"Failed to initialize SSH client",
				"host", sshOpts.Host,
				"user", sshOpts.User,
				"port", sshOpts.Port,
				"ssh_key", sshOpts.KeyPath,
				"use_ssh_agent", sshOpts.UseAgent,
				"error", err.Error(),
			)
			os.Exit(1)
		}
		defer installer.SshClient.Close()

		slog.Info(
			"Ensuring Valkey is installed and configured",
			"host", sshOpts.Host,
			"user", valkeyUsername,
			"bind", valkeyBind,
			"port", valkeyPort,
		)

		if err := installer.EnsureInstalledAndConfigured(valkeyUsername, valkeyPassword, valkeyBind, valkeyPort, valkeyACLFile); err != nil {
			slog.Error("Failed to configure Valkey", "error", err.Error())
			os.Exit(1)
		}

		slog.Info(
			"Valkey installation and remote access setup completed",
			"host", sshOpts.Host,
			"user", valkeyUsername,
			"bind", valkeyBind,
			"port", valkeyPort,
		)

		fmt.Printf("Valkey host: %s\n", sshOpts.Host)
		fmt.Printf("Valkey port: %d\n", valkeyPort)
		fmt.Printf("Valkey user: %s\n", valkeyUsername)
		fmt.Printf("Valkey URI: %s\n", buildValkeyURL(sshOpts.Host, valkeyPort, valkeyUsername, valkeyPassword))
	},
}

func init() {
	valkeyNewViper = viper.New()

	databaseValkeyCmd.AddCommand(databaseValkeyNewCmd)

	databaseValkeyNewCmd.Flags().String("username", "", "Valkey ACL username to create or update")
	databaseValkeyNewCmd.Flags().String("password", "", "Valkey ACL password to create or update")
	databaseValkeyNewCmd.Flags().String("bind", "0.0.0.0", "Address for Valkey to bind to for remote access")
	databaseValkeyNewCmd.Flags().Int("port", 6379, "Valkey TCP port")
	databaseValkeyNewCmd.Flags().String("acl-file", "", "Path to the Valkey ACL file; defaults based on the detected config path")

	valkeyNewViper.BindPFlag("valkey_username", databaseValkeyNewCmd.Flags().Lookup("username"))
	valkeyNewViper.BindPFlag("valkey_password", databaseValkeyNewCmd.Flags().Lookup("password"))
	valkeyNewViper.BindPFlag("valkey_bind", databaseValkeyNewCmd.Flags().Lookup("bind"))
	valkeyNewViper.BindPFlag("valkey_port", databaseValkeyNewCmd.Flags().Lookup("port"))
	valkeyNewViper.BindPFlag("valkey_acl_file", databaseValkeyNewCmd.Flags().Lookup("acl-file"))
}

func buildValkeyURL(host string, port int, username, password string) string {
	return fmt.Sprintf(
		"redis://%s:%s@%s:%d",
		urlQueryEscape(username),
		urlQueryEscape(password),
		host,
		port,
	)
}

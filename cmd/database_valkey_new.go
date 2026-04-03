package cmd

import (
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
		sshHost := rootViperCfg.GetString("ssh_remote_host")
		sshUser := rootViperCfg.GetString("ssh_remote_user")
		sshKey := expandPath(rootViperCfg.GetString("ssh_key"))
		sshPassphrase := rootViperCfg.GetString("ssh_passphrase")
		useSshAgent := rootViperCfg.GetBool("ssh_use_agent")
		sshPort := rootViperCfg.GetUint("ssh_port")

		valkeyUsername := valkeyNewViper.GetString("valkey_username")
		valkeyPassword := valkeyNewViper.GetString("valkey_password")
		valkeyBind := valkeyNewViper.GetString("valkey_bind")
		valkeyPort := valkeyNewViper.GetInt("valkey_port")
		valkeyACLFile := valkeyNewViper.GetString("valkey_acl_file")

		if sshHost == "" {
			slog.Error("SSH host is required", "hint", "set the global --ssh-remote-host flag")
			os.Exit(1)
		}
		if sshUser == "" {
			sshUser = currentUserName()
		}
		if sshKey == "" {
			sshKey = defaultSSHKeyPath()
		}
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
			sshHost,
			sshUser,
			sshKey,
			sshPassphrase,
			useSshAgent,
			sshPort,
		)
		if err != nil {
			slog.Error(
				"Failed to initialize SSH client",
				"host", sshHost,
				"user", sshUser,
				"port", sshPort,
				"ssh_key", sshKey,
				"use_ssh_agent", useSshAgent,
				"error", err.Error(),
			)
			os.Exit(1)
		}
		defer installer.SshClient.Close()

		slog.Info(
			"Ensuring Valkey is installed and configured",
			"host", sshHost,
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
			"host", sshHost,
			"user", valkeyUsername,
			"bind", valkeyBind,
			"port", valkeyPort,
		)
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

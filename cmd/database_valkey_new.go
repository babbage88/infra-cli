package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/babbage88/infra-cli/tui"
	coredeploy "github.com/babbage88/infra-core/deployment"
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
			valkeyUsername = tui.Input("Valkey ACL username", valkeyUsername)
		}
		if !cmd.Flags().Changed("password") {
			valkeyPassword = tui.Password("Valkey ACL password", valkeyPassword)
		}

		slog.Info(
			"Ensuring Valkey is installed and configured",
			"host", sshOpts.Host,
			"user", valkeyUsername,
			"bind", valkeyBind,
			"port", valkeyPort,
		)

		result, err := coredeploy.InstallValkey(coredeploy.ValkeyInstallRequest{
			SSH: coredeploy.SSHOptions{
				Host:       sshOpts.Host,
				User:       sshOpts.User,
				KeyPath:    sshOpts.KeyPath,
				Passphrase: sshOpts.Passphrase,
				UseAgent:   sshOpts.UseAgent,
				Port:       sshOpts.Port,
			},
			Username: valkeyUsername,
			Password: valkeyPassword,
			Bind:     valkeyBind,
			Port:     valkeyPort,
			ACLFile:  valkeyACLFile,
		})
		if err != nil {
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

		fmt.Printf("Valkey host: %s\n", result.Host)
		fmt.Printf("Valkey port: %d\n", result.Port)
		fmt.Printf("Valkey user: %s\n", result.Username)
		fmt.Printf("Valkey URI: %s\n", result.URI)
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
	return coredeploy.BuildValkeyURL(host, port, username, password)
}

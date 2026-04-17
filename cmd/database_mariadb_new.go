package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/babbage88/infra-cli/deployer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var mariaDBNewViper *viper.Viper

var databaseMariaDBNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Install and configure MariaDB for remote access on a host over SSH",
	Run: func(cmd *cobra.Command, args []string) {
		sshHost := rootViperCfg.GetString("ssh_remote_host")
		sshUser := rootViperCfg.GetString("ssh_remote_user")
		sshKey := expandPath(rootViperCfg.GetString("ssh_key"))
		sshPassphrase := rootViperCfg.GetString("ssh_passphrase")
		useSshAgent := rootViperCfg.GetBool("ssh_use_agent")
		sshPort := rootViperCfg.GetUint("ssh_port")

		dbName := mariaDBNewViper.GetString("db_name")
		dbUser := mariaDBNewViper.GetString("db_user")
		dbPassword := mariaDBNewViper.GetString("db_password")
		mariaDBBind := mariaDBNewViper.GetString("mariadb_bind")
		mariaDBPort := mariaDBNewViper.GetInt("mariadb_port")

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
		if mariaDBPort <= 0 {
			slog.Error("Invalid MariaDB port", "port", mariaDBPort)
			os.Exit(1)
		}

		dbName, dbUser, dbPassword = promptForMissingAppConfig(cmd, dbName, dbUser, dbPassword)

		installer, err := deployer.NewRemoteMariaDBInstallerWithSsh(
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
			"Ensuring MariaDB is installed and configured",
			"host", sshHost,
			"database", dbName,
			"user", dbUser,
			"bind", mariaDBBind,
			"port", mariaDBPort,
		)

		if err := installer.EnsureInstalledAndConfigured(dbName, dbUser, dbPassword, mariaDBBind, mariaDBPort); err != nil {
			slog.Error("Failed to configure MariaDB", "error", err.Error())
			os.Exit(1)
		}

		slog.Info(
			"MariaDB installation and remote access setup completed",
			"host", sshHost,
			"database", dbName,
			"user", dbUser,
			"bind", mariaDBBind,
			"port", mariaDBPort,
		)

		fmt.Printf("MariaDB host: %s\n", sshHost)
		fmt.Printf("MariaDB port: %d\n", mariaDBPort)
		fmt.Printf("MariaDB database: %s\n", dbName)
		fmt.Printf("MariaDB user: %s\n", dbUser)
		fmt.Printf("MariaDB URI: %s\n", buildMariaDBURL(sshHost, mariaDBPort, dbName, dbUser, dbPassword))
	},
}

func init() {
	mariaDBNewViper = viper.New()

	databaseMariaDBCmd.AddCommand(databaseMariaDBNewCmd)

	databaseMariaDBNewCmd.Flags().String("db-name", "", "Name of the database to create/configure")
	databaseMariaDBNewCmd.Flags().String("db-user", "", "Service user name to create")
	databaseMariaDBNewCmd.Flags().String("db-password", "", "Password for the service user")
	databaseMariaDBNewCmd.Flags().String("bind", "0.0.0.0", "Address for MariaDB to bind to for remote access")
	databaseMariaDBNewCmd.Flags().Int("port", 3306, "MariaDB TCP port")

	mariaDBNewViper.BindPFlag("db_name", databaseMariaDBNewCmd.Flags().Lookup("db-name"))
	mariaDBNewViper.BindPFlag("db_user", databaseMariaDBNewCmd.Flags().Lookup("db-user"))
	mariaDBNewViper.BindPFlag("db_password", databaseMariaDBNewCmd.Flags().Lookup("db-password"))
	mariaDBNewViper.BindPFlag("mariadb_bind", databaseMariaDBNewCmd.Flags().Lookup("bind"))
	mariaDBNewViper.BindPFlag("mariadb_port", databaseMariaDBNewCmd.Flags().Lookup("port"))
}

func buildMariaDBURL(host string, port int, dbname, username, password string) string {
	return fmt.Sprintf(
		"mysql://%s:%s@%s:%d/%s",
		urlQueryEscape(username),
		urlQueryEscape(password),
		host,
		port,
		urlQueryEscape(dbname),
	)
}

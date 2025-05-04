package cmd

import (
	"fmt"
	"log/slog"

	"github.com/babbage88/infra-cli/deployer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var pgInstallViper viper.Viper

var databaseInstallPgCmd = &cobra.Command{
	Use:   "install-pg",
	Short: "Install Postgres and configure for remote connections",
	RunE: func(cmd *cobra.Command, args []string) error {
		pgInstaller, err := deployer.NewRemotePostgresInstallerWithSsh(pgInstallViper.GetString("pg_hostname"),
			rootViperCfg.GetString("ssh_remote_user"),
			rootViperCfg.GetString("ssh_key"),
			rootViperCfg.GetString("ssh_passphrase"),
			rootViperCfg.GetBool("ssh_use_agent"),
			rootViperCfg.GetUint("ssh_port"))

		if err != nil {
			return err
		}

		out, err := pgInstaller.SshClient.Run("cat /etc/os-release")
		if err != nil {
			return err
		}
		pgInstaller.ParseOsReleaseContent(out)
		slog.Info("", slog.String("Name", pgInstaller.OsInfo["NAME"]))
		fmt.Println(string(out))
		return nil
	},
}

func init() {
	pgInstallViper = *viper.New()
	//curUser, _ := getCurrentUserName()
	var newPgPass string
	var RemoteHostName string

	databaseCmd.AddCommand(databaseInstallPgCmd)
	databaseInstallPgCmd.Flags().StringVar(&newPgPass, "postgres-password", "", "The password to set for the postgres user.")
	databaseInstallPgCmd.Flags().StringVar(&RemoteHostName, "hostname", "", "Remote hostname8 to install postgres.")

	pgInstallViper.BindPFlag("postgres_password", databaseInstallPgCmd.Flags().Lookup("postgres-password"))
	pgInstallViper.BindPFlag("pg_hostname", databaseInstallPgCmd.Flags().Lookup("hostname"))

}

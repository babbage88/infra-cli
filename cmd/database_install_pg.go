package cmd

import "github.com/spf13/cobra"

var databaseInstallPgCmd = &cobra.Command{
	Use:   "install-pg",
	Short: "Install Postgres and configure for remote connections",
}

func init() {
	//curUser, _ := getCurrentUserName()
	var newPgPass string
	var RemoteHostName string

	rootCmd.AddCommand(databaseInstallPgCmd)
	databaseInstallPgCmd.Flags().StringVar(&newPgPass, "postgres-password", "", "The password to set for the postgres user.")
	databaseInstallPgCmd.Flags().StringVar(&RemoteHostName, "hostname", "", "Remote hostname8 to install postgres.")

	rootViperCfg.BindPFlag("postgres_password", databaseInstallPgCmd.PersistentFlags().Lookup("postgres-password"))
	rootViperCfg.BindPFlag("pg_hostname", databaseInstallPgCmd.PersistentFlags().Lookup("hostname"))

}

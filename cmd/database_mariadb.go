package cmd

import "github.com/spf13/cobra"

var databaseMariaDBCmd = &cobra.Command{
	Use:   "mariadb",
	Short: "Manage MariaDB instances",
}

func init() {
	databaseCmd.AddCommand(databaseMariaDBCmd)
}

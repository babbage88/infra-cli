package cmd

import (
	"github.com/babbage88/infra-cli/tui"
	"github.com/spf13/cobra"
)

func promptForMissingAppConfig(cmd *cobra.Command, dbname, username, password string) (string, string, string) {
	if !cmd.Flags().Changed("db-name") {
		dbname = tui.Input("Database name", dbname)
	}
	if !cmd.Flags().Changed("db-user") {
		username = tui.Input("Database user", username)
	}
	if !cmd.Flags().Changed("db-password") {
		password = tui.Password("Database password", password)
	}

	return dbname, username, password
}

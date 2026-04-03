package cmd

import "github.com/spf13/cobra"

var databaseValkeyCmd = &cobra.Command{
	Use:   "valkey",
	Short: "Manage Valkey instances",
}

func init() {
	databaseCmd.AddCommand(databaseValkeyCmd)
}

package cmd

import "github.com/spf13/cobra"

var databaseCmd = &cobra.Command{
	Use:   "database",
	Short: "Manage databases",
}

func init() {
	rootCmd.AddCommand(databaseCmd)
}

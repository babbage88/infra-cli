package cmd

import "github.com/spf13/cobra"

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Manage storage services",
}

func init() {
	rootCmd.AddCommand(storageCmd)
}

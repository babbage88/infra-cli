package cmd

import "github.com/spf13/cobra"

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deployment-related commands",
}

func init() {
	rootCmd.AddCommand(deployCmd)
}

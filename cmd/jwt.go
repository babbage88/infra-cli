package cmd

import "github.com/spf13/cobra"

var jwtCmd = &cobra.Command{
	Use:   "jwt",
	Short: "JWT authentication commands",
}

func init() {
	authCmd.AddCommand(jwtCmd)
}

package cmd

import "github.com/spf13/cobra"

var certificatesCmd = &cobra.Command{
	Use:   "certificates",
	Short: "Generate and trust development certificates",
}

func init() {
	rootCmd.AddCommand(certificatesCmd)
}

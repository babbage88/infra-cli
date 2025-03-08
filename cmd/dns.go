package cmd

import "github.com/spf13/cobra"

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "DNS management commands",
}

func init() {
	rootCmd.AddCommand(dnsCmd)
}

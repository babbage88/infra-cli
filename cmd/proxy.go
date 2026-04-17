package cmd

import "github.com/spf13/cobra"

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Install and configure reverse proxies on remote hosts over SSH",
}

func init() {
	rootCmd.AddCommand(proxyCmd)
}

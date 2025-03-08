package cmd

import "github.com/spf13/cobra"

var cloudflareCmd = &cobra.Command{
	Use:   "cloudflare",
	Short: "Manage Cloudflare DNS records",
}

func init() {
	dnsCmd.AddCommand(cloudflareCmd)
}

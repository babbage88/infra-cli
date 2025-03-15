package cmd

import (
	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/spf13/cobra"
)

var cloudflareCmd = &cobra.Command{
	Use:   "cloudflare",
	Short: "Manage Cloudflare DNS records",
	Run: func(cmd *cobra.Command, args []string) {
		for _, record := range recordsBatch.Records {
			pretty.Print("The following records were parsed from dns-config:\n")

			pretty.Printf("ZoneName: %s", record.ZoneName)
			pretty.Printf("Name: %s", record.Name)
			pretty.Printf("Content: %s", record.Content)
		}
	},
}

var (
	cfDnsToken string
)

func init() {
	dnsCmd.AddCommand(cloudflareCmd)
}

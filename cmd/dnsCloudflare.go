package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cloudflareCmd = &cobra.Command{
	Use:   "cloudflare",
	Short: "Manage Cloudflare DNS records",
}

var (
	cfDnsToken string
)

func init() {
	dnsCmd.AddCommand(cloudflareCmd)
	//Checking if api token for cloudflare was passed via apiTokens from root command.
	cfapi, exists := apiTokens["cloudflare"]
	if exists {
		cfDnsToken = cfapi
	} else {
		viper.SetEnvPrefix("cf")
		viper.AutomaticEnv()
	}

}

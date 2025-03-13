package cmd

import (
	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	//Checking if api token for cloudflare was passed via apiTokens from root command.
	cobra.OnInitialize(func() {
		apiTokens = viper.GetStringMapString("api_tokens")
		cfapi, exists := apiTokens["cloudflare"]
		if exists {
			cfDnsToken = cfapi
		} else {
			viper.SetEnvPrefix("cf")
			viper.AutomaticEnv()
		}

		if dnsCfgFile != "" {
			if err := dnsCfg.Unmarshal(&recordsBatch); err != nil {
				pretty.PrintErrorf("Unable to decode into struct: %v", err)
			}
		}

	})

}

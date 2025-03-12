package cmd

import (
	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/spf13/cobra"
)

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "DNS management commands",
}

func init() {
	rootCmd.AddCommand(dnsCmd)
	dnsCmd.PersistentFlags().StringVar(&dnsCfgFile, "dns-config", "",
		"Config file (default is default.yaml)")
	err := mergeDnsConfigFile()
	if err != nil {
		pretty.PrintErrorf("error merging dns-config %s", err.Error())
	}
}

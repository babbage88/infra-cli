package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/babbage88/infra-cli/providers/cloudflare_utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var zoneCmd = &cobra.Command{
	Use:     "zone",
	Aliases: []string{"get-zoneid"},
	Short:   "Get the zoneId for a given domain-name Cloudflare DNS records",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println()
		err := getZoneIdCmd(cfDnsToken, viper.GetString("zone_name"))
		if err != nil {
			pretty.PrettyErrorLogF("Error initializing cloudflare api client %s\n", err.Error())
			return err
		}
		return err
	},
}

func init() {
	cloudflareCmd.AddCommand(zoneCmd)
	zoneCmd.PersistentFlags().String("zone-name", viper.GetString("domain_name"), "DNS Zone Name to fetch ID for.")
	viper.BindPFlag("zone_name", zoneCmd.PersistentFlags().Lookup("zone-name"))
}

func getZoneIdCmd(token string, zoneName string) error {
	zoneId, err := cloudflare_utils.GetCloudFlareZoneIdByDomainName(token, zoneName)
	if err != nil {
		pretty.PrettyErrorLogF("Error retrieving DNS Records %s", err.Error())
		return err
	}
	switch rawFlag {
	case true:
		printDnsAndZoneIdTable(zoneName, zoneId)
	default:
		prettyPrintZoneIdTable(zoneName, zoneId)
	}

	return nil
}

func printDnsAndZoneIdTable(domain string, zoneId string) error {
	tw := tabwriter.NewWriter(os.Stdout, 5, 0, 1, ' ', tabwriter.TabIndent)
	fmt.Fprintln(tw, "ZoneName\t\tZoneID")
	fmt.Fprintln(tw, "----------\t\t------")
	fmt.Fprintf(tw, "%s\t\t%s\n", domain, zoneId)
	err := tw.Flush()
	return err
}

func prettyPrintZoneIdTable(domain string, zoneId string) error {
	var colorInt int32 = 92
	coloStartSting := fmt.Sprintf("\x1b[1;%dm", colorInt)
	colorEndString := "\x1b[0m"
	tw := tabwriter.NewWriter(os.Stdout, 5, 0, 1, ' ', tabwriter.TabIndent)
	fmt.Fprintf(tw, "%sZoneName\tZoneID%s\n", coloStartSting, colorEndString)
	fmt.Fprintf(tw, "%s--------\t------%s\n", coloStartSting, colorEndString)
	fmt.Fprintf(tw, "%s%s\t%s%s\n", coloStartSting, domain, zoneId, colorEndString)
	err := tw.Flush()
	return err
}

package cmd

import (
	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create Cloudflare DNS records",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := recordsBatch.GetAllZoneIdsForBatch(cfDnsToken)
		if err != nil {
			pretty.PrintErrorf("error retrieving zoneIds err: %s", err.Error())
			return err
		}
		err = recordsBatch.ProcessDnsBatch(cfDnsToken)
		if err != nil {
			pretty.PrintErrorf("error retrieving zoneIds err: %s", err.Error())
			return err
		}
		return err
	},
}

func init() {
	var recordFile string
	cloudflareCmd.AddCommand(createCmd)
	createCmd.Flags().StringVar(&recordFile, "file", "dnsRecords.yaml", "Path to YAML file that contains the new record details.")
	createCmd.Flags().String("zone-id", "", "Cloudflare Zone ID")
	createCmd.Flags().String("record-name", "", "DNS Record Name")
	createCmd.Flags().String("record-type", "", "DNS Record Type")
	createCmd.Flags().String("content", "", "DNS Record Content")
	createCmd.Flags().Int("ttl", 120, "DNS Record TTL (default: 120)")
	createCmd.Flags().Bool("proxied", false, "Enable Cloudflare proxy (default: false)")
	createCmd.Flags().String("priority", "", "DNS Record Priority (for MX/SRV records)")
	createCmd.Flags().String("comment", "", "DNS Record Comment")

	// Bind flags to viper for flexibility
	viper.BindPFlag("zone_id", createCmd.Flags().Lookup("zone-id"))
	viper.BindPFlag("record_name", createCmd.Flags().Lookup("record-name"))
	viper.BindPFlag("record_type", createCmd.Flags().Lookup("record-type"))
	viper.BindPFlag("content", createCmd.Flags().Lookup("content"))
	viper.BindPFlag("ttl", createCmd.Flags().Lookup("ttl"))
	viper.BindPFlag("proxied", createCmd.Flags().Lookup("proxied"))
	viper.BindPFlag("priority", createCmd.Flags().Lookup("priority"))
	viper.BindPFlag("comment", createCmd.Flags().Lookup("comment"))
	nameProvided := createCmd.Flags().Changed("record_name")
	contentProvided := createCmd.Flags().Changed("content")
	recTypeProvided := createCmd.Flags().Changed("record_type")
	enoughFlagsForCreate := nameProvided && contentProvided && recTypeProvided
	viper.Set("parse_flags", enoughFlagsForCreate)
	cobra.OnInitialize(func() {
		if apiTokens != nil {
			cfapi, exists := apiTokens["cloudflare"]
			if exists {
				cfDnsToken = cfapi
			}
		}

		flagRecord := parseFlags(createCmd)
		if flagRecord != nil {
			recordsBatch.Records = append(recordsBatch.Records, *flagRecord)
		}
	})
}

// overrideFlags ensures command-line flag values take precedence over YAML file values
func parseFlags(cmd *cobra.Command) *DnsRecord {
	flagRecord := &DnsRecord{}
	if viper.GetBool("parse_flags") {
		flagRecord.Name, _ = cmd.Flags().GetString("record-name")
		flagRecord.Type, _ = cmd.Flags().GetString("record-type")
		flagRecord.Content, _ = cmd.Flags().GetString("content")
		if cmd.Flags().Changed("ttl") {
			flagRecord.TTL, _ = cmd.Flags().GetInt("ttl")
		} else {
			flagRecord.TTL = 120
		}
		if cmd.Flags().Changed("proxied") {
			flagRecord.Proxied, _ = cmd.Flags().GetBool("proxied")
		} else {
			flagRecord.Proxied = true
		}
		if cmd.Flags().Changed("priority") {
			ptr, _ := cmd.Flags().GetUint16("priority")
			flagRecord.Priority = &ptr
		}
		if cmd.Flags().Changed("comment") {
			flagRecord.Comment, _ = cmd.Flags().GetString("comment")
		}
		return flagRecord
	}
	return nil
}

func WhereAmI(param any) *any {
	ptr := &param
	return ptr
}

package cmd

import (
	"fmt"
	"os"

	"github.com/cloudflare/cloudflare-go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var recordFile string
var dnsRecord cloudflare.CreateDNSRecordParams

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create Cloudflare DNS records",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load from YAML if provided
		if recordFile != "" {
			viper.SetConfigFile(recordFile)
			if err := viper.ReadInConfig(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read record file: %v\n", err)
				return err
			}
			if err := viper.Unmarshal(&dnsRecord); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to parse YAML: %v\n", err)
				return err
			}
		}

		// Override YAML values with command-line flags
		overrideFlags(cmd)

		// Validate required fields
		if dnsRecord.ZoneID == "" || dnsRecord.Name == "" || dnsRecord.Type == "" || dnsRecord.Content == "" {
			return fmt.Errorf("zone-id, record-name, record-type, and content are required")
		}

		// Execute creation logic
		fmt.Printf("Creating DNS Record: %+v\n", dnsRecord)
		return nil
	},
}

func init() {
	dnsCmd.AddCommand(createCmd)

	createCmd.Flags().StringVar(&recordFile, "file", "", "Path to DNS record YAML file")
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
}

// overrideFlags ensures command-line flag values take precedence over YAML file values
func overrideFlags(cmd *cobra.Command) {
	if cmd.Flags().Changed("zone-id") {
		dnsRecord.ZoneID, _ = cmd.Flags().GetString("zone-id")
	}
	if cmd.Flags().Changed("record-name") {
		dnsRecord.Name, _ = cmd.Flags().GetString("record-name")
	}
	if cmd.Flags().Changed("record-type") {
		dnsRecord.Type, _ = cmd.Flags().GetString("record-type")
	}
	if cmd.Flags().Changed("content") {
		dnsRecord.Content, _ = cmd.Flags().GetString("content")
	}
	if cmd.Flags().Changed("ttl") {
		dnsRecord.TTL, _ = cmd.Flags().GetInt("ttl")
	}
	if cmd.Flags().Changed("proxied") {
		dnsRecord.Proxied, _ = cmd.Flags().GetBool("proxied")
	}
	if cmd.Flags().Changed("priority") {
		dnsRecord.Priority, _ = cmd.Flags().GetString("priority")
	}
	if cmd.Flags().Changed("comment") {
		dnsRecord.Comment, _ = cmd.Flags().GetString("comment")
	}
}

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var proxmoxVmListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all VMs on a Proxmox node",
	RunE: func(cmd *cobra.Command, args []string) error {
		// initialize a local viper.Viper instance
		localViper := viper.New()

		// If the --config-file flage argument has been specified,
		// parse those values into localViper instance
		if cmd.Flags().Changed("config-file") {
			cfgFile, _ := cmd.Flags().GetString("config-file")
			if cfgFile != "" {
				err := loadProxmoxConfigFile(cfgFile, localViper)
				if err != nil {
					slog.Error("Failed to load config", "error", err.Error())
					return err
				}
			}
		}

		// bind local cmd.Flags() to localViper, overriding any config-file supplied value
		bindLocalFlags(cmd, localViper)
		ctx := context.Background()

		client, err := newProxmoxClientFromViperConfig(localViper)
		if err != nil {
			slog.Error("failed to create proxmox client", slog.String("url", proxmoxApiUrl), "error", err.Error())
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Fetch VMs
		vms, err := client.ListVMs(ctx, proxPveNodeFlagVar)
		if err != nil {
			return fmt.Errorf("error retrieving VM list: %w", err)
		}

		// Pretty print JSON
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(vms); err != nil {
			return fmt.Errorf("failed to encode VM list: %w", err)
		}

		return nil
	},
}

func init() {
	proxmoxVmSubCmd.AddCommand(proxmoxVmListCmd)

	// Config file flag
	proxmoxVmListCmd.Flags().StringVar(&configFilePath, "config-file", "", "Path to YAML config file for PVE auth")

	// Auth flags
	proxmoxVmListCmd.Flags().StringVarP(&proxoxUserFlagVar, "username", "u", "root", "Username or Auth token name")
	proxmoxVmListCmd.Flags().StringVar(&proxmoxPasswordFlagVar, "password", "", "Password for user or Auth token")
	proxmoxVmListCmd.Flags().BoolVar(&proxmoxApiAuthBoolVar, "use-token", true, "Use API token authentication")
	proxmoxVmListCmd.Flags().BoolVar(&proxmoxIgnoreTLSErrorBoolVar, "skip-tls", true, "Skip TLS/SSL certificate validation")
	proxmoxVmListCmd.Flags().StringVar(&proxmoxAuthToken, "proxmox-api-token", "", "Proxmox API token ID")
	proxmoxVmListCmd.Flags().StringVar(&proxmoxAuthTokenSecret, "proxmox-api-secret", "", "Proxmox API token secret")

	// Node flags
	proxmoxVmListCmd.Flags().StringVar(&proxmoxApiUrl, "proxmox-api-url", "", "Proxmox api url")
	proxmoxVmListCmd.Flags().StringVar(&proxPveNodeFlagVar, "pve-node", "proxmox3", "Proxmox node name")
	proxmoxVmListCmd.Flags().IntVar(&proxPortFlagVar, "pve-port", 8006, "Proxmox PVE port")
}

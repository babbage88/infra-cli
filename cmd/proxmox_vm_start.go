package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var proxmoxVmStartSubCmd = &cobra.Command{
	Use:   "get",
	Short: "Command for updating a Proxmox VM's configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		localViper := viper.New()

		cfgFile, _ := cmd.Flags().GetString("config-file")
		if cfgFile != "" {
			err := loadProxmoxConfigFile(cfgFile, localViper)
			if err != nil {
				slog.Error("Failed to load config", "error", err.Error())
				os.Exit(1)
			}
		}

		bindLocalFlags(cmd, localViper)
		ctx := context.Background()

		client, err := newProxmoxClientFromViperConfig(localViper)
		if err != nil {
			slog.Error("failed to create proxmox client", slog.String("url", proxmoxApiUrl), "error", err.Error())
			return err
		}

		// Loop through all VMIDs
		for _, vmid := range proxVmIdFlagVar {
			vmInfo, err := client.GetVMConfig(ctx, proxPveNodeFlagVar, vmid)
			if err != nil {
				slog.Error("error retrieving vm info", slog.String("node", proxPveNodeFlagVar), slog.Int("vmid", vmid), "error", err.Error())
				continue // skip errors but keep going
			}
			fmt.Printf("\n=== VMID %d ===\n", vmid)
			vmInfo.PrettyPrintJSON()
			client.StartVM(ctx, proxPveNodeFlagVar, vmid)
		}

		return nil
	},
}

func init() {
	proxmoxVmSubCmd.AddCommand(proxmoxVmStartSubCmd)
	proxmoxVmStartSubCmd.Flags().StringVar(&configFilePath, "config-file", "", "Path to YAML config file for PVE auth and vm")

	// Auth
	proxmoxVmStartSubCmd.Flags().StringVarP(&proxmoxApiUrl, "url", "u", "", "Proxmox API URL")
	proxmoxVmStartSubCmd.Flags().StringVar(&proxoxUserFlagVar, "username", "root", "Username or Auth token name")
	proxmoxVmStartSubCmd.Flags().StringVar(&proxmoxPasswordFlagVar, "password", "", "Password for user or Auth token")
	proxmoxVmStartSubCmd.Flags().StringVar(&rootCAPathFlagVar, "rootca-path", "", "RootCA path for TLS validation")
	proxmoxVmStartSubCmd.Flags().BoolVar(&proxmoxApiAuthBoolVar, "use-token", true, "Use API token authentication")
	proxmoxVmStartSubCmd.Flags().BoolVar(&proxmoxIgnoreTLSErrorBoolVar, "skip-tls", true, "Skip TLS/SSL certificate validation")
	proxmoxVmStartSubCmd.Flags().StringVar(&proxmoxAuthToken, "proxmox-api-token", "", "Proxmox API token ID")
	proxmoxVmStartSubCmd.Flags().StringVar(&proxmoxAuthTokenSecret, "proxmox-api-secret", "", "Proxmox API token secret")

	// VM flags
	proxmoxVmStartSubCmd.Flags().StringVar(&proxPveNodeFlagVar, "pve-node", "proxmox3", "Proxmox node name")
	proxmoxVmStartSubCmd.Flags().IntVar(&proxPortFlagVar, "pve-port", 8006, "Proxmox PVE Port")
	proxmoxVmStartSubCmd.Flags().IntSliceVar(&proxVmIdFlagVar, "vmid", nil, "One or more VMIDs to retrieve")
	proxmoxVmStartSubCmd.Flags().StringVar(&proxVmNameFlagVar, "name", "", "Name for the Proxmox VM")
}

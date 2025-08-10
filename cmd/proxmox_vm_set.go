package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/babbage88/infra-cli/proxmox"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var updateVmIDs []int

var proxmoxVmUpdateListCmd = &cobra.Command{
	Use:     "set",
	Aliases: []string{"update", "update-list"},
	Short:   "Update configuration for multiple Proxmox VMs",
	RunE: func(cmd *cobra.Command, args []string) error {
		localViper := viper.New()

		cfgFile, _ := cmd.Flags().GetString("config-file")
		if cfgFile != "" {
			if err := loadProxmoxConfigFile(cfgFile, localViper); err != nil {
				slog.Error("Failed to load config", "error", err.Error())
				os.Exit(1)
			}
		}
		bindLocalFlags(cmd, localViper)

		ctx := context.Background()

		pveUser := localViper.GetString("username")
		pveSecret := localViper.GetString("password")
		if proxmoxApiAuthBoolVar {
			pveUser = localViper.GetString("proxmox_api_token")
			pveSecret = localViper.GetString("proxmox_api_secret")
		}

		if proxmoxApiUrl == "" {
			proxmoxApiUrl = fmt.Sprintf("https://%s:%d", localViper.GetString("pve_node"), localViper.GetInt("pve_port"))
		}

		client, err := proxmox.NewClient(
			proxmoxApiUrl,
			pveUser,
			pveSecret,
			proxmoxIgnoreTLSErrorBoolVar,
			proxmoxApiAuthBoolVar,
		)
		if err != nil {
			return fmt.Errorf("failed to create proxmox client: %w", err)
		}

		// Build a config containing only explicitly set flags
		vmCfg, err := buildVMConfigFromCmd(cmd)
		if err != nil {
			return fmt.Errorf("building VM config: %w", err)
		}

		// If user didn't set any flags to change, abort
		if vmCfg.Name == "" && vmCfg.MemoryMB == "" && vmCfg.Sockets == "" && vmCfg.Cores == "" && vmCfg.Description == "" && len(vmCfg.Raw) == 0 {
			return fmt.Errorf("no VM configuration flags provided; nothing to update")
		}

		// Update each VMID
		for _, vmid := range updateVmIDs {
			slog.Info("Updating VM", "vmid", vmid)
			if err := client.UpdateVMConfig(ctx, proxPveNodeFlagVar, vmid, vmCfg); err != nil {
				slog.Error("Failed to update VM", "vmid", vmid, "error", err.Error())
			} else {
				slog.Info("Successfully updated VM", "vmid", vmid)
				_ = vmCfg.PrintJSON()
			}
		}

		return nil
	},
}

func init() {
	proxmoxVmSubCmd.AddCommand(proxmoxVmUpdateListCmd)
	proxmoxVmUpdateListCmd.Flags().StringVar(&configFilePath, "config-file", "", "Path to YAML config file for PVE auth and vm")

	// Auth
	proxmoxVmUpdateListCmd.Flags().StringVarP(&proxmoxApiUrl, "url", "u", "", "Proxmox API URL")
	proxmoxVmUpdateListCmd.Flags().StringVar(&proxoxUserFlagVar, "username", "root", "Username or Auth token name")
	proxmoxVmUpdateListCmd.Flags().StringVar(&proxmoxPasswordFlagVar, "password", "", "Password for user or Auth token")
	proxmoxVmUpdateListCmd.Flags().StringVar(&rootCAPathFlagVar, "rootca-path", "", "RootCA path for TLS validation")
	proxmoxVmUpdateListCmd.Flags().BoolVar(&proxmoxApiAuthBoolVar, "use-token", false, "Use API token authentication")
	proxmoxVmUpdateListCmd.Flags().BoolVar(&proxmoxIgnoreTLSErrorBoolVar, "skip-tls", true, "Skip TLS/SSL certificate validation")
	proxmoxVmUpdateListCmd.Flags().StringVar(&proxmoxAuthToken, "proxmox-api-token", "", "Proxmox API token ID")
	proxmoxVmUpdateListCmd.Flags().StringVar(&proxmoxAuthTokenSecret, "proxmox-api-secret", "", "Proxmox API token secret")

	// VM Config
	proxmoxVmUpdateListCmd.Flags().StringVar(&proxPveNodeFlagVar, "pve-node", "proxmox3", "Proxmox node name")
	proxmoxVmUpdateListCmd.Flags().IntVar(&proxPortFlagVar, "pve-port", 8006, "Proxmox PVE Port")
	proxmoxVmUpdateListCmd.Flags().IntSliceVar(&updateVmIDs, "vmids", []int{}, "Comma-separated list of VMIDs to update")
	proxmoxVmUpdateListCmd.Flags().StringVar(&proxVmNameFlagVar, "name", "", "Name for the Proxmox VM")
	proxmoxVmUpdateListCmd.Flags().IntVar(&proxVmSocketFlagVar, "sockets", 1, "Number of sockets")
	proxmoxVmUpdateListCmd.Flags().IntVar(&proxVmCoresFlagVar, "cores", 1, "CPU cores")
	proxmoxVmUpdateListCmd.Flags().IntVar(&proxMemowryFlagVar, "memory", 1024, "Memory in MB")
	proxmoxVmUpdateListCmd.Flags().StringVar(&proxVmDescFlagVar, "description", "Development VM", "Description for the Proxmox VM")
}

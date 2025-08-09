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

var (
	updateVmIDs []int
)

var proxmoxVmUpdateListCmd = &cobra.Command{
	Use:     "update",
	Aliases: []string{"set", "update-list"},
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

		// Parse VM config from flags
		vmCfgCmd := ProxmoxVmCreateCommand{
			Name:        proxVmNameFlagVar,
			MemoryMB:    proxMemowryFlagVar,
			Sockets:     proxVmSocketFlagVar,
			Cores:       proxVmCoresFlagVar,
			Description: proxVmDescFlagVar,
		}
		vmCfg := vmCfgCmd.ParseVMConfigTyped()

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
	proxmoxVmUpdateListCmd.Flags().StringVar(&proxPveNodeFlagVar, "pve-node", "proxmox3", "Proxmox node name")
	proxmoxVmUpdateListCmd.Flags().IntSliceVar(&updateVmIDs, "vmids", []int{}, "Comma-separated list of VMIDs to update")
	proxmoxVmUpdateListCmd.Flags().StringVar(&proxVmNameFlagVar, "name", "", "Name for the Proxmox VM")
	proxmoxVmUpdateListCmd.Flags().IntVar(&proxVmSocketFlagVar, "sockets", 1, "Number of sockets")
	proxmoxVmUpdateListCmd.Flags().IntVar(&proxVmCoresFlagVar, "cores", 1, "CPU cores")
	proxmoxVmUpdateListCmd.Flags().IntVar(&proxMemowryFlagVar, "memory", 1024, "Memory in MB")
	proxmoxVmUpdateListCmd.Flags().StringVar(&proxVmDescFlagVar, "description", "Development VM", "Description for the Proxmox VM")
}

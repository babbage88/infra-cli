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
	createVmID int
)

var proxmoxVmCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new", "add"},
	Short:   "Create a new Proxmox VM",
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

		vmCfgCmd := ProxmoxVmCreateCommand{
			Name:        proxVmNameFlagVar,
			MemoryMB:    proxMemowryFlagVar,
			Sockets:     proxVmSocketFlagVar,
			Cores:       proxVmCoresFlagVar,
			Description: proxVmDescFlagVar,
		}
		vmCfg := vmCfgCmd.ParseVMConfigTyped()

		if err := client.CreateVM(ctx, proxPveNodeFlagVar, createVmID, vmCfg); err != nil {
			return fmt.Errorf("failed to create VM: %w", err)
		}

		slog.Info("VM created successfully", "vmid", createVmID)
		_ = vmCfg.PrintJSON()
		return nil
	},
}

func init() {
	proxmoxVmSubCmd.AddCommand(proxmoxVmCreateCmd)
	proxmoxVmCreateCmd.Flags().StringVar(&configFilePath, "config-file", "", "Path to YAML config file for PVE auth and vm")
	proxmoxVmCreateCmd.Flags().StringVar(&proxPveNodeFlagVar, "pve-node", "proxmox3", "Proxmox node name")
	proxmoxVmCreateCmd.Flags().IntVar(&createVmID, "vmid", 0, "VMID to assign to the new VM (must be unique)")
	proxmoxVmCreateCmd.Flags().StringVar(&proxVmNameFlagVar, "name", "", "Name for the Proxmox VM")
	proxmoxVmCreateCmd.Flags().IntVar(&proxVmSocketFlagVar, "sockets", 1, "Number of sockets")
	proxmoxVmCreateCmd.Flags().IntVar(&proxVmCoresFlagVar, "cores", 1, "CPU cores")
	proxmoxVmCreateCmd.Flags().IntVar(&proxMemowryFlagVar, "memory", 1024, "Memory in MB")
	proxmoxVmCreateCmd.Flags().StringVar(&proxVmDescFlagVar, "description", "New VM", "Description for the Proxmox VM")
}

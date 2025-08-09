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

var proxmoxVmGetSubCmd = &cobra.Command{
	Use:   "get",
	Short: "Command for updating a Proxmox VM's configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		localViper := viper.New()
		var (
			pveUser   string
			pveSecret string
		)
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

		if proxmoxApiAuthBoolVar {
			pveUser = localViper.GetString("proxmox_api_token")
			pveSecret = localViper.GetString("proxmox_api_secret")
		} else {
			pveUser = localViper.GetString("username")
			pveSecret = localViper.GetString("password")
		}

		if proxmoxApiUrl == "" {
			proxmoxApiUrl = fmt.Sprintf("https://%s:%d", localViper.GetString("pve_node"), localViper.GetInt("pve_port"))
		}
		slog.Info("Proxmox API URL", "url", proxmoxApiUrl)

		client, err := proxmox.NewClient(
			proxmoxApiUrl,
			pveUser,
			pveSecret,
			proxmoxIgnoreTLSErrorBoolVar,
			proxmoxApiAuthBoolVar,
		)
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
			vmInfo.PrintJSON()
		}
		return nil
	},
}

func init() {
	proxmoxVmSubCmd.AddCommand(proxmoxVmGetSubCmd)
	proxmoxVmGetSubCmd.Flags().StringVar(&configFilePath, "config-file", "", "Path to YAML config file for PVE auth and vm")

	// Auth
	proxmoxVmGetSubCmd.Flags().StringVarP(&proxmoxApiUrl, "url", "u", "", "Proxmox API URL")
	proxmoxVmGetSubCmd.Flags().StringVar(&proxoxUserFlagVar, "username", "root", "Username or Auth token name")
	proxmoxVmGetSubCmd.Flags().StringVar(&proxmoxPasswordFlagVar, "password", "", "Password for user or Auth token")
	proxmoxVmGetSubCmd.Flags().StringVar(&rootCAPathFlagVar, "rootca-path", "", "RootCA path for TLS validation")
	proxmoxVmGetSubCmd.Flags().BoolVar(&proxmoxApiAuthBoolVar, "use-token", false, "Use API token authentication")
	proxmoxVmGetSubCmd.Flags().BoolVar(&proxmoxIgnoreTLSErrorBoolVar, "skip-tls", true, "Skip TLS/SSL certificate validation")
	proxmoxVmGetSubCmd.Flags().StringVar(&proxmoxAuthToken, "proxmox-api-token", "", "Proxmox API token ID")
	proxmoxVmGetSubCmd.Flags().StringVar(&proxmoxAuthTokenSecret, "proxmox-api-secret", "", "Proxmox API token secret")

	// VM flags
	proxmoxVmGetSubCmd.Flags().StringVar(&proxPveNodeFlagVar, "pve-node", "proxmox3", "Proxmox node name")
	proxmoxVmGetSubCmd.Flags().IntVar(&proxPortFlagVar, "pve-port", 8006, "Proxmox PVE Port")
	proxmoxVmGetSubCmd.Flags().IntSliceVar(&proxVmIdFlagVar, "vmid", nil, "One or more VMIDs to retrieve")
	proxmoxVmGetSubCmd.Flags().StringVar(&proxVmNameFlagVar, "name", "", "Name for the Proxmox VM")
}

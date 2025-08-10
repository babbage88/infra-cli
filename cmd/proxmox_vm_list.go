package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/babbage88/infra-cli/proxmox"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var proxmoxListCommandFlags ProxmoxVmCommandFlags

var proxmoxVmListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all VMs on a Proxmox node",
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

		if proxmoxListCommandFlags.ApiUrl == "" {
			proxmoxApiUrl = fmt.Sprintf("https://%s:%d", localViper.GetString("pve_node"), localViper.GetInt("pve_port"))
		} else {
			proxmoxApiUrl = localViper.GetString("proxmox_api_url")
		}
		slog.Info("Proxmox API URL", "url", proxmoxApiUrl)

		client, err := proxmox.NewClient(
			proxmoxApiUrl,
			pveUser,
			pveSecret,
			proxmoxListCommandFlags.SkipTls,
			proxmoxListCommandFlags.UseToken,
		)
		if err != nil {
			slog.Error("failed to create proxmox client", slog.String("url", proxmoxApiUrl), "error", err.Error())
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Fetch VMs
		vms, err := client.ListVMs(ctx, proxmoxListCommandFlags.PveNode)
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

	// Auth flags
	proxmoxVmListCmd.Flags().StringVar(&configFilePath, "config-file", "", "Path to YAML config file for PVE auth")
	proxmoxVmListCmd.Flags().StringVarP(&proxmoxListCommandFlags.AuthTokenOrUsername, "username", "u", "root", "Username or Auth token name")
	proxmoxVmListCmd.Flags().StringVar(&proxmoxListCommandFlags.PasswordOrSecret, "password", "", "Password for user or Auth token")
	proxmoxVmListCmd.Flags().BoolVar(&proxmoxListCommandFlags.UseToken, "use-token", false, "Use API token authentication")
	proxmoxVmListCmd.Flags().BoolVar(&proxmoxListCommandFlags.SkipTls, "skip-tls", true, "Skip TLS/SSL certificate validation")
	proxmoxVmListCmd.Flags().StringVar(&proxmoxListCommandFlags.AuthTokenOrUsername, "proxmox-api-token", "", "Proxmox API token ID")
	proxmoxVmListCmd.Flags().StringVar(&proxmoxListCommandFlags.PasswordOrSecret, "proxmox-api-secret", "", "Proxmox API token secret")

	// Node flags
	proxmoxVmListCmd.Flags().StringVar(&proxmoxListCommandFlags.ApiUrl, "proxmox-api-url", "", "Proxmox api url")
	proxmoxVmListCmd.Flags().StringVar(&proxmoxListCommandFlags.PveNode, "pve-node", "proxmox3", "Proxmox node name")
	proxmoxVmListCmd.Flags().StringVar(&proxmoxListCommandFlags.PvePort, "pve-port", "8006", "Proxmox PVE port")
}

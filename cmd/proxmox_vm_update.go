package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/babbage88/infra-cli/proxmox"
	"github.com/spf13/cobra"
)

var (
	proxPveNodeFlagVar           string
	proxVmIdFlagVar              int
	proxPortFlagVar              int
	proxVmSocketFlagVar          int
	proxVmCoresFlagVar           int
	proxMemowryFlagVar           int
	proxoxUserFlagVar            string
	proxmoxPasswordFlagVar       string
	proxVmNameFlagVar            string
	proxVmDescFlagVar            string
	proxmoxApiUrl                string
	proxmoxAuthToken             string
	proxmoxAuthTokenSecret       string
	proxmoxApiAuthBoolVar        bool
	proxmoxIgnoreTLSErrorBoolVar bool
	rootCAPathFlagVar            string
)

var proxmoxVmSetSubCmd = &cobra.Command{
	Use:   "get",
	Short: "Command for updating a Proxmox VM's configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// If API URL not provided, construct it correctly
		if proxmoxApiUrl == "" {
			proxmoxApiUrl = fmt.Sprintf("https://%s:%d", proxPveNodeFlagVar, proxPortFlagVar)
		}
		slog.Info("Proxmox API URL", "url", proxmoxApiUrl)

		var (
			err    error
			client *proxmox.Client
		)

		if proxmoxApiAuthBoolVar {
			client, err = proxmox.NewClientToken(
				proxmoxApiUrl,
				proxmoxAuthToken,       // e.g. "root@pam!mytoken"
				proxmoxAuthTokenSecret, // secret value
				proxmoxIgnoreTLSErrorBoolVar,
			)
		} else {
			client, err = proxmox.NewClientPassword(
				proxmoxApiUrl,
				proxoxUserFlagVar,
				proxmoxPasswordFlagVar,
				proxmoxIgnoreTLSErrorBoolVar,
			)
		}

		if err != nil {
			slog.Error("failed to create proxmox client", slog.String("url", proxmoxApiUrl), "error", err.Error())
			return err
		}

		vmInfo, err := client.GetVMConfig(ctx, proxPveNodeFlagVar, proxVmIdFlagVar)
		if err != nil {
			slog.Error("error retrieving vm info from Proxmox API", slog.String("node", proxPveNodeFlagVar), "error", err.Error())
			return err
		}

		fmt.Printf("VM Info: %+v\n", vmInfo)
		return nil
	},
}

func init() {
	proxmoxVmSubCmd.AddCommand(proxmoxVmSetSubCmd)

	// Auth
	proxmoxVmSetSubCmd.Flags().StringVarP(&proxmoxApiUrl, "url", "u", "", "Username or Auth token name")
	proxmoxVmSetSubCmd.Flags().StringVar(&proxoxUserFlagVar, "username", "root", "Username or Auth token name")
	proxmoxVmSetSubCmd.Flags().StringVar(&proxmoxPasswordFlagVar, "password", "", "Password for user or Auth token")
	proxmoxVmSetSubCmd.Flags().StringVar(&rootCAPathFlagVar, "rootca-path", "", "RootCA path for TLS validation")
	proxmoxVmSetSubCmd.Flags().BoolVar(&proxmoxApiAuthBoolVar, "use-token", false, "Password for user or Auth token")
	proxmoxVmSetSubCmd.Flags().BoolVar(&proxmoxApiAuthBoolVar, "skip-tls", true, "Skip TLS/SSL certificate validation")
	proxmoxVmSetSubCmd.Flags().StringVar(&proxmoxAuthToken, "token", "", "Proxmox API token ID")
	proxmoxVmSetSubCmd.Flags().StringVar(&proxmoxAuthTokenSecret, "secret", "", "Proxmox API token secret")

	// VM Set flags
	proxmoxVmSetSubCmd.Flags().StringVar(&proxPveNodeFlagVar, "pve-node", "proxmox3", "Proxmox node name")
	proxmoxVmSetSubCmd.Flags().IntVar(&proxPortFlagVar, "pve-port", 8006, "Proxmox PVE Port. Default: 8006")
	proxmoxVmSetSubCmd.Flags().IntVar(&proxVmIdFlagVar, "vmid", 106, "The VMID to update")
	proxmoxVmSetSubCmd.Flags().StringVar(&proxVmNameFlagVar, "name", "", "Name for the Proxmox VM")
	proxmoxVmSetSubCmd.Flags().IntVar(&proxVmSocketFlagVar, "sockets", 1, "Storage for container")
	proxmoxVmSetSubCmd.Flags().IntVar(&proxVmCoresFlagVar, "cores", 1, "CPU cores")
	proxmoxVmSetSubCmd.Flags().IntVar(&proxMemowryFlagVar, "memory", 1024, "Memory in MB")
	proxmoxVmSetSubCmd.Flags().StringVar(&proxVmDescFlagVar, "Description", "Development VM", "Description for the Proxmox VM")
}

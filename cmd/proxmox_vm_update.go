package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/babbage88/infra-cli/proxmox"
	"github.com/spf13/cobra"
)

var (
	proxPveNodeFlagVar     string
	proxVmIdFlagVar        int
	proxPortFlagVar        int
	proxVmSocketFlagVar    int
	proxVmCoresFlagVar     int
	proxMemowryFlagVar     int
	proxoxUserFlagVar      string
	proxmoxPasswordFlagVar string
	proxVmNameFlagVar      string
	proxVmDescFlagVar      string
	proxmoxApiUrl          string
	proxmoxAuthToken       string
	proxmoxAuthTokenSecret string
	proxmoxApiAuthBoolVar  bool
)

var proxmoxVmSetSubCmd = &cobra.Command{
	Use:   "vm",
	Short: "Comman for updating a Proxmox VM's configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		if proxmoxApiUrl == "" {
			proxmoxApiUrl = fmt.Sprintf("https://%s:%s", proxPveNodeFlagVar, proxmoxPasswordFlagVar)
		}
		var err error
		var client *proxmox.Client
		if proxmoxApiAuthBoolVar {
			client, err = proxmox.NewClientToken(proxmoxApiUrl, proxmoxAuthToken, proxmoxAuthTokenSecret)
		} else {
			client, err = proxmox.NewClientPassword(proxmoxApiUrl, proxoxUserFlagVar, proxmoxPasswordFlagVar)
		}
		vmInfo, err := client.GetVMConfig(ctx, proxPveNodeFlagVar, proxVmIdFlagVar)
		if err != nil {
			slog.Error("error retrieving vm info from Rpoxmox API", slog.String("node", proxPveNodeFlagVar), "error", err.Error())
			return err
		}
		fmt.Println(vmInfo)
		return err
	},
}

func init() {
	proxmoxVmSubCmd.AddCommand(proxmoxVmSetSubCmd)

	// Auth
	proxmoxVmSubCmd.Flags().StringVarP(&proxmoxApiUrl, "url", "u", "", "Username or Auth token name")
	proxmoxVmSubCmd.Flags().StringVar(&proxoxUserFlagVar, "username", "root", "Username or Auth token name")
	proxmoxVmSubCmd.Flags().StringVar(&proxmoxPasswordFlagVar, "password", "", "Password for user or Auth token")
	proxmoxVmSubCmd.Flags().BoolVar(&proxmoxApiAuthBoolVar, "use-token", false, "Password for user or Auth token")
	proxmoxVmSubCmd.Flags().StringVar(&proxmoxAuthToken, "token", "", "Proxmox API token ID")
	proxmoxVmSubCmd.Flags().StringVar(&proxmoxAuthTokenSecret, "secret", "", "Proxmox API token secret")

	// VM Set flags
	proxmoxVmSubCmd.Flags().StringVar(&proxPveNodeFlagVar, "pve-node", "10.0.0.9", "Proxmox node name")
	proxmoxVmSubCmd.Flags().IntVar(&proxPortFlagVar, "pve-port", 8006, "Proxmox PVE Port. Default: 8006")
	proxmoxVmSubCmd.Flags().IntVar(&proxVmIdFlagVar, "vmid", 9090, "The VMID to update")
	proxmoxVmSubCmd.Flags().StringVar(&proxVmNameFlagVar, "name", "", "Name for the Proxmox VM")
	proxmoxVmSubCmd.Flags().IntVar(&proxVmSocketFlagVar, "sockets", 1, "Storage for container")
	proxmoxVmSubCmd.Flags().IntVar(&proxVmCoresFlagVar, "cores", 1, "CPU cores")
	proxmoxVmSubCmd.Flags().IntVar(&proxMemowryFlagVar, "memory", 1024, "Memory in MB")
	proxmoxVmSubCmd.Flags().StringVar(&proxVmDescFlagVar, "Description", "Development VM", "Description for the Proxmox VM")
}

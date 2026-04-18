package cmd

import "github.com/spf13/cobra"

var proxmoxAuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Inspect Proxmox authentication and authorization details",
}

func init() {
	proxmoxSubCmd.AddCommand(proxmoxAuthCmd)
}

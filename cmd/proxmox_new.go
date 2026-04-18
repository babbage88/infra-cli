package cmd

import "github.com/spf13/cobra"

var proxmoxNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Create new Proxmox users and API tokens",
}

func init() {
	proxmoxSubCmd.AddCommand(proxmoxNewCmd)
}

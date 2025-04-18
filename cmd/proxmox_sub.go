package cmd

import "github.com/spf13/cobra"

var proxmoxSubCmd = &cobra.Command{
	Use:     "proxmox",
	Aliases: []string{"pve"},
	Short:   "Commands for creation and management of proxmox resources like lxc containers or virtual machine",
}

func init() {
	rootCmd.AddCommand(proxmoxSubCmd)
}

package cmd

import "github.com/spf13/cobra"

var proxmoxVmSubCmd = &cobra.Command{
	Use:   "vm",
	Short: "Commands for creation and management of Proxmox (QEMU) VMs",
}

func init() {
	proxmoxSubCmd.AddCommand(proxmoxVmSubCmd)
}

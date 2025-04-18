package cmd

import "github.com/spf13/cobra"

var proxmoxLxcSubCmd = &cobra.Command{
	Use:   "lxc",
	Short: "Commands for creation and management of lxc containers",
}

func init() {
	proxmoxSubCmd.AddCommand(proxmoxLxcSubCmd)
}

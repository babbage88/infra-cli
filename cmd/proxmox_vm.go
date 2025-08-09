package cmd

import "github.com/spf13/cobra"

var (
	proxPveNodeFlagVar           string
	proxVmIdFlagVar              []int
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

var proxmoxVmSubCmd = &cobra.Command{
	Use:   "vm",
	Short: "Commands for creation and management of Proxmox (QEMU) VMs",
}

func init() {
	proxmoxSubCmd.AddCommand(proxmoxVmSubCmd)
}

package cmd

import (
	"github.com/babbage88/infra-cli/proxmox"
	"github.com/spf13/cobra"
)

var newLxcRequest proxmox.LxcContainer
var proxmoxLxcAuth proxmox.Auth

var proxmoxLxcCreateCmd = &cobra.Command{
	Use:     "create-lxc",
	Aliases: []string{"new-lxc", "create", "new"},
	Short:   "Commands for creation and management of lxc containers",
}

func init() {
	proxmoxLxcSubCmd.AddCommand(proxmoxLxcCreateCmd)
	proxmoxLxcCreateCmd.Flags().StringVar(&proxmoxLxcAuth.Host, "host-url", "", "Proxmox host url to send request to.")
	proxmoxLxcCreateCmd.Flags().StringVar(&proxmoxLxcAuth.ApiToken, "api-token", "", "Proxmox API token.")

	proxmoxLxcCreateCmd.Flags().IntVar(&newLxcRequest.VmId, "vmid", 9090, "vmid of new container")
	proxmoxLxcCreateCmd.Flags().StringVar(&newLxcRequest.Node, "pve-node", "", "Name of proxmox node where lxc container will be created")
	proxmoxLxcCreateCmd.Flags().StringVar(&newLxcRequest.Password, "lxc-password", "", "New container password")
	proxmoxLxcCreateCmd.Flags().StringVar(&newLxcRequest.Ostemplate, "ostemplate", "local:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst", "New lxc ostemplate")
	proxmoxLxcCreateCmd.Flags().StringVar(&newLxcRequest.Storage, "storage", "local-lvm", "Where lxc ct will be stored")
	proxmoxLxcCreateCmd.Flags().StringVar(&newLxcRequest.RootFsSize, "rootfs-size", "9G", "Root volume size")
	proxmoxLxcCreateCmd.Flags().IntVar(&newLxcRequest.Memory, "memory", 1024, "Memory size")
	proxmoxLxcCreateCmd.Flags().IntVar(&newLxcRequest.Swap, "swap", 512, "Swap size")
	proxmoxLxcCreateCmd.Flags().IntVar(&newLxcRequest.Cores, "cores", 1, "Number of cores")
	proxmoxLxcCreateCmd.Flags().IntVar(&newLxcRequest.CpuLimit, "cpu-limit", 0, "Cpu limit")
	proxmoxLxcCreateCmd.Flags().IntVar(&newLxcRequest.CpuUnits, "cpu-units", 1024, "Cpu Units")
	proxmoxLxcCreateCmd.Flags().StringVar(&newLxcRequest.Net0, "net0", "name=eth0,bridge=vmbr0,ip=dhcp,type=veth", "Network interface config")
	proxmoxLxcCreateCmd.Flags().IntVar(&newLxcRequest.Unprivileged, "unprivileged", 1, "1 - unprivileged, 0 - privileged")
	proxmoxLxcCreateCmd.Flags().IntVar(&newLxcRequest.Start, "start", 1, "1 - start after creation, 0 - leave off")
	proxmoxLxcCreateCmd.Flags().IntVar(&newLxcRequest.Console, "console", 1, "1 - attach console, 0 - no console")

}

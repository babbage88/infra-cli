package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/babbage88/infra-cli/proxmox"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var newLxcRequest proxmox.LxcContainer
var proxmoxLxcAuth proxmox.Auth

var proxmoxLxcCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new-lxc", "create-lxc", "new"},
	Short:   "Create a new LXC container on a Proxmox node",
	Run: func(cmd *cobra.Command, args []string) {
		localViper := viper.New()
		// Step 1: Load config file first
		cfgFile, _ := cmd.Flags().GetString("config-file")
		if cfgFile != "" {
			err := loadProxmoxConfigFile(cfgFile, localViper)

			if err != nil {
				log.Fatalf("Failed to load config: %v", err)
			}
		}
		fmt.Println("DEBUG: Viper config loaded:", localViper.AllSettings())

		// Step 2: Bind flags AFTER config is loaded
		bindLocalFlags(cmd, localViper)

		// Bind vp. values into the struct manually
		proxmoxLxcAuth.Host = localViper.GetString("host_url")
		proxmoxLxcAuth.ApiToken = localViper.GetString("api_token")

		newLxcRequest.VmId = localViper.GetInt("vmid")
		newLxcRequest.Node = localViper.GetString("pve_node")
		newLxcRequest.Password = localViper.GetString("lxc_password")
		newLxcRequest.OsTemplate = localViper.GetString("ostemplate")
		newLxcRequest.Storage = localViper.GetString("storage")
		newLxcRequest.RootFsSize = localViper.GetString("rootfs_size")
		newLxcRequest.Memory = localViper.GetInt("memory")
		newLxcRequest.Swap = localViper.GetInt("swap")
		newLxcRequest.Cores = localViper.GetInt("cores")
		newLxcRequest.CpuLimit = localViper.GetInt("cpu_limit")
		newLxcRequest.CpuUnits = localViper.GetInt("cpu_units")
		newLxcRequest.Net0 = localViper.GetString("net0")

		newLxcRequest.Arch = localViper.GetString("arch")
		newLxcRequest.Cmode = localViper.GetString("cmode")

		if localViper.GetBool("start") {
			newLxcRequest.Start = "1"
		} else {
			newLxcRequest.Start = "0"
		}

		if localViper.GetBool("console") {
			newLxcRequest.Console = "1"
		} else {
			newLxcRequest.Console = "0"
		}

		if localViper.GetBool("unprivileged") {
			newLxcRequest.Unprivileged = "1"
		} else {
			newLxcRequest.Unprivileged = "0"
		}

		newLxcRequest.SshPublicKeys = localViper.GetString("ssh_public_keys")
		if strings.HasPrefix(newLxcRequest.SshPublicKeys, "@") {
			filePath := strings.TrimPrefix(newLxcRequest.SshPublicKeys, "@")
			content, err := os.ReadFile(filePath)
			if err != nil {
				log.Fatalf("Failed to read ssh public keys file: %v", err)
			}
			newLxcRequest.SshPublicKeys = string(content)
		}

		fmt.Println("Creating LXC container...")
		params := newLxcRequest.ToFormParams()
		err := proxmox.CreateLXCContainer(proxmoxLxcAuth.Host, proxmoxLxcAuth.ApiToken, newLxcRequest.Node, params)
		if err != nil {
			log.Fatalf("Error creating container: %v", err)
		}
		fmt.Println("Container creation request sent successfully.")
	},
}

func init() {
	proxmoxLxcSubCmd.AddCommand(proxmoxLxcCreateCmd)

	proxmoxLxcCreateCmd.Flags().StringVar(&configFilePath, "config-file", "", "Path to YAML config file containing proxmox lxc info")

	// Auth flags
	proxmoxLxcCreateCmd.Flags().String("host-url", "", "Proxmox host URL")
	proxmoxLxcCreateCmd.Flags().String("api-token", "", "Proxmox API token")

	// LXC flags
	proxmoxLxcCreateCmd.Flags().Int("vmid", 9090, "Container VM ID")
	proxmoxLxcCreateCmd.Flags().String("pve-node", "", "Proxmox node name")
	proxmoxLxcCreateCmd.Flags().String("lxc-password", "", "Container root password")
	proxmoxLxcCreateCmd.Flags().String("ostemplate", "local:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst", "OS template")
	proxmoxLxcCreateCmd.Flags().String("ssh-public-keys", "", "Authorized SSH public keys (as string or @filepath)")
	proxmoxLxcCreateCmd.Flags().String("storage", "local-lvm", "Storage for container")
	proxmoxLxcCreateCmd.Flags().String("rootfs-size", "9G", "Root filesystem size")
	proxmoxLxcCreateCmd.Flags().Int("memory", 1024, "Memory in MB")
	proxmoxLxcCreateCmd.Flags().Int("swap", 512, "Swap in MB")
	proxmoxLxcCreateCmd.Flags().Int("cores", 1, "CPU cores")
	proxmoxLxcCreateCmd.Flags().Int("cpu-limit", 0, "CPU limit")
	proxmoxLxcCreateCmd.Flags().Int("cpu-units", 1024, "CPU weight")
	proxmoxLxcCreateCmd.Flags().String("net0", "name=eth0,bridge=vmbr0,ip=dhcp,type=veth", "Network config")
	proxmoxLxcCreateCmd.Flags().Bool("unprivileged", true, "Use unprivileged container")
	proxmoxLxcCreateCmd.Flags().Bool("start", true, "Start after create")
	proxmoxLxcCreateCmd.Flags().Bool("console", true, "Attach console")

}

func bindLocalFlags(cmd *cobra.Command, vp *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		key := strings.ReplaceAll(f.Name, "-", "_")
		_ = vp.BindPFlag(key, f)
	})
}

func loadProxmoxConfigFile(path string, vp *viper.Viper) error {
	if path != "" {
		vp.SetConfigFile(path)
		if err := vp.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		apiCheck := vp.GetString("api_token")
		err := validateProxmoxApiToken(apiCheck, vp)
		if err != nil {
			return fmt.Errorf("error validatin api token %w", err)
		}
		return nil
	}

	return nil
}

func validateProxmoxApiToken(apiCheck string, vp *viper.Viper) error {
	if apiCheck == "" {
		apiTokenFromRoot := rootViperCfg.GetString("proxmox_api_token")
		switch len(apiTokenFromRoot) {
		case 0:
			return fmt.Errorf("no proxmox api token supplied")
		default:
			vp.Set("api_token", apiTokenFromRoot)
			return nil
		}
	} else {
		return nil
	}
}

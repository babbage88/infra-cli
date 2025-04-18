package cmd

import (
	"fmt"
	"log"
	"log/slog"
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
	Use:     "create-lxc",
	Aliases: []string{"new-lxc", "create", "new"},
	Short:   "Create a new LXC container on a Proxmox node",
	Run: func(cmd *cobra.Command, args []string) {
		// Bind viper values into the struct manually
		proxmoxLxcAuth.Host = viper.GetString("host_url")
		proxmoxLxcAuth.ApiToken = viper.GetString("api_token")

		newLxcRequest.VmId = viper.GetInt("vmid")
		newLxcRequest.Node = viper.GetString("pve_node")
		newLxcRequest.Password = viper.GetString("lxc_password")
		newLxcRequest.OsTemplate = viper.GetString("ostemplate")
		newLxcRequest.Storage = viper.GetString("storage")
		newLxcRequest.RootFsSize = viper.GetString("rootfs_size")
		newLxcRequest.Memory = viper.GetInt("memory")
		newLxcRequest.Swap = viper.GetInt("swap")
		newLxcRequest.Cores = viper.GetInt("cores")
		newLxcRequest.CpuLimit = viper.GetInt("cpu_limit")
		newLxcRequest.CpuUnits = viper.GetInt("cpu_units")
		newLxcRequest.Net0 = viper.GetString("net0")
		newLxcRequest.Unprivileged = viper.GetInt("unprivileged")
		newLxcRequest.Start = viper.GetInt("start")
		newLxcRequest.Console = viper.GetInt("console")

		newLxcRequest.SshPublicKeys = viper.GetString("ssh_public_keys")
		if strings.HasPrefix(newLxcRequest.SshPublicKeys, "@") {
			filePath := strings.TrimPrefix(newLxcRequest.SshPublicKeys, "@")
			content, err := os.ReadFile(filePath)
			if err != nil {
				log.Fatalf("Failed to read ssh public keys file: %v", err)
			}
			newLxcRequest.SshPublicKeys = string(content)
		}

		fmt.Println("Creating LXC container...")
		err := proxmox.CreateLxcContainer(proxmoxLxcAuth, newLxcRequest)
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
	proxmoxLxcCreateCmd.Flags().Int("unprivileged", 1, "Use unprivileged container (1=yes, 0=no)")
	proxmoxLxcCreateCmd.Flags().Int("start", 1, "Start after create (1=yes, 0=no)")
	proxmoxLxcCreateCmd.Flags().Int("console", 1, "Attach console (1=yes, 0=no)")

	// Bind all flags to viper
	viperBindFlags(proxmoxLxcCreateCmd)
	proxCfgPath := viper.GetString("config-file")
	if proxCfgPath != "" {
		err := loadProxmoxConfigFile(proxCfgPath)
		if err != nil {
			slog.Error("Error loading config-file", slog.String("path", proxCfgPath), "error", err.Error())
			os.Exit(1)
		}
	}
}

func viperBindFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		viperKey := strings.ReplaceAll(f.Name, "-", "_")
		_ = viper.BindPFlag(viperKey, f)
	})
}

func loadProxmoxConfigFile(path string) error {
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	apiCheck := viper.GetString("api_token")
	err := validateProxmoxApiToken(apiCheck)
	if err != nil {
		return fmt.Errorf("error validatin api token %w", err)
	}
	return nil
}

func validateProxmoxApiToken(apiCheck string) error {
	if apiCheck == "" {
		apiTokenFromRoot := rootViperCfg.GetString("proxmox_api_token")
		switch len(apiTokenFromRoot) {
		case 0:
			return fmt.Errorf("no proxmox api token supplied")
		default:
			viper.Set("api_token", apiTokenFromRoot)
			return nil
		}
	} else {
		return nil
	}
}

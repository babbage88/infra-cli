package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/babbage88/infra-cli/proxmox"
	infraSSH "github.com/babbage88/infra-cli/ssh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	newLxcRequest  proxmox.LxcContainer
	proxmoxLxcAuth proxmox.Auth
)

var proxmoxLxcCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new-lxc", "create-lxc", "new"},
	Short:   "Create a new LXC container on a Proxmox node",
	RunE: func(cmd *cobra.Command, args []string) error {
		localViper := viper.New()
		cfgFile, _ := cmd.Flags().GetString("config-file")
		if cfgFile != "" {
			err := loadProxmoxConfigFile(cfgFile, localViper)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
		}

		bindLocalFlags(cmd, localViper)
		applyRootProxmoxDefaults(localViper)

		proxmoxLxcAuth.Host = localViper.GetString("host_url")
		proxmoxLxcAuth.ApiToken = localViper.GetString("api_token")

		newLxcRequest = proxmox.LxcContainer{
			VmId:          localViper.GetInt("vmid"),
			Hostname:      localViper.GetString("lxc_hostname"),
			Node:          localViper.GetString("pve_node"),
			Password:      localViper.GetString("lxc_password"),
			OsTemplate:    localViper.GetString("ostemplate"),
			Storage:       localViper.GetString("storage"),
			RootFsSize:    localViper.GetString("rootfs_size"),
			Memory:        localViper.GetInt("memory"),
			Swap:          localViper.GetInt("swap"),
			Cores:         localViper.GetInt("cores"),
			CpuLimit:      localViper.GetInt("cpu_limit"),
			CpuUnits:      localViper.GetInt("cpu_units"),
			Net0:          localViper.GetString("net0"),
			Arch:          localViper.GetString("arch"),
			Cmode:         localViper.GetString("cmode"),
			SshPublicKeys: localViper.GetStringSlice("ssh_public_keys"),
		}
		newLxcRequest.Start = boolToProxmoxFlag(localViper.GetBool("start"))
		newLxcRequest.Console = boolToProxmoxFlag(localViper.GetBool("console"))
		newLxcRequest.Unprivileged = boolToProxmoxFlag(localViper.GetBool("unprivileged"))

		if err := promptForMissingLxcCreateBasics(cmd, localViper, &proxmoxLxcAuth, &newLxcRequest); err != nil {
			return err
		}

		client, err := proxmox.NewClientTokenString(proxmoxLxcAuth.Host, proxmoxLxcAuth.ApiToken, true)
		if err != nil {
			return fmt.Errorf("create proxmox client: %w", err)
		}
		if err := promptForMissingLxcCreateTemplateAndStorage(cmd, localViper, client, &newLxcRequest); err != nil {
			return err
		}

		fmt.Println("Creating LXC container...")
		if err := client.CreateLXCContainer(context.Background(), newLxcRequest.Node, &newLxcRequest); err != nil {
			return fmt.Errorf("create container: %w", err)
		}
		fmt.Println("Container creation request sent successfully.")
		return nil
	},
}

func applyRootProxmoxDefaults(vp *viper.Viper) {
	normalizeProxmoxConfigValues(vp)
	if vp.GetString("host_url") == "" {
		if rootURL := rootViperCfg.GetString("proxmox_api_url"); rootURL != "" {
			vp.Set("host_url", rootURL)
		} else if rootURL := rootViperCfg.GetString("proxmox_host"); rootURL != "" {
			vp.Set("host_url", rootURL)
		}
	}
	if vp.GetString("api_token") == "" {
		if rootToken, rootSecret := resolveConfiguredProxmoxTokenParts(rootViperCfg); rootToken != "" && rootSecret != "" {
			vp.Set("api_token", fmt.Sprintf("%s=%s", rootToken, rootSecret))
		}
	}
}

func boolToProxmoxFlag(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func promptForMissingLxcCreateBasics(cmd *cobra.Command, vp *viper.Viper, auth *proxmox.Auth, req *proxmox.LxcContainer) error {
	if strings.TrimSpace(req.Node) == "" {
		req.Node = promptInputWithExample("Proxmox node", "pve01", "")
	}
	if strings.TrimSpace(auth.Host) == "" {
		defaultHostURL := defaultProxmoxHostURL(req.Node)
		auth.Host = promptInputWithExample("Proxmox host URL", "https://proxmox.example.com:8006", defaultHostURL)
	}
	if shouldPromptForProxmoxAPIAuth(cmd, vp, auth.ApiToken) {
		auth.ApiToken = promptForProxmoxAPIAuthToken()
	}
	if !cmd.Flags().Changed("vmid") && !vp.InConfig("vmid") {
		req.VmId = mustPromptLxcID(auth, req.Node, req.VmId, 9090)
	} else if req.VmId <= 0 {
		req.VmId = mustPromptLxcID(auth, req.Node, req.VmId, 9090)
	}
	if strings.TrimSpace(req.Hostname) == "" {
		req.Hostname = promptInputWithExample("Container hostname", "app-staging-01", "")
	}
	if len(req.SshPublicKeys) == 0 && strings.TrimSpace(req.Password) == "" {
		req.Password = promptPasswordWithExample("Container root password", "correct-horse-battery-staple", "")
	}

	return nil
}

func shouldPromptForProxmoxAPIAuth(cmd *cobra.Command, vp *viper.Viper, currentValue string) bool {
	if strings.TrimSpace(currentValue) == "" {
		return true
	}

	return !cmd.Flags().Changed("api-token") && !vp.InConfig("api_token")
}

func defaultProxmoxHostURL(node string) string {
	node = strings.TrimSpace(node)
	if node == "" {
		return ""
	}

	return fmt.Sprintf("https://%s:8006", node)
}

func promptForProxmoxAPIAuthToken() string {
	if defaults, source, ok := rootConfiguredProxmoxTokenParts(); ok {
		if source != "" {
			fmt.Printf("Found Proxmox API token and secret in %s.\n", source)
		} else {
			fmt.Println("Found Proxmox API token and secret in your root config.")
		}
		if promptYesNo("Use the configured Proxmox API token and secret?", true) {
			return combinedProxmoxToken(defaults)
		}
		return combinedProxmoxToken(promptForProxmoxTokenParts(defaults))
	}

	return combinedProxmoxToken(promptForProxmoxTokenParts(proxmoxTokenParts{}))
}

func promptForMissingLxcCreateTemplateAndStorage(cmd *cobra.Command, vp *viper.Viper, client *proxmox.Client, req *proxmox.LxcContainer) error {
	if !cmd.Flags().Changed("storage") && !vp.InConfig("storage") && strings.TrimSpace(req.Storage) == "" {
		req.Storage = promptInputWithExample("Container storage", "local-lvm", "local-lvm")
	} else if strings.TrimSpace(req.Storage) == "" {
		req.Storage = promptInputWithExample("Container storage", "local-lvm", "local-lvm")
	}
	if !cmd.Flags().Changed("rootfs-size") && !vp.InConfig("rootfs_size") && strings.TrimSpace(req.RootFsSize) == "" {
		req.RootFsSize = promptInputWithExample("Root filesystem size in GB", "16", "9")
	} else if strings.TrimSpace(req.RootFsSize) == "" {
		req.RootFsSize = promptInputWithExample("Root filesystem size in GB", "16", "9")
	}
	if shouldPromptForOSTemplate(cmd, vp, req.OsTemplate) {
		templateOptions, err := listAvailableLxcTemplates(req.Node, client)
		if err != nil {
			slog.Warn("failed to list LXC templates automatically; falling back to manual input", "error", err.Error())
			req.OsTemplate = promptInputWithExample("OS template", "local:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst", req.OsTemplate)
			return nil
		}
		req.OsTemplate = promptSelectOption("Select OS template", templateOptions, req.OsTemplate)
	}

	return nil
}

func shouldPromptForOSTemplate(cmd *cobra.Command, vp *viper.Viper, currentValue string) bool {
	if strings.TrimSpace(currentValue) == "" {
		return true
	}

	defaultTemplate := cmd.Flags().Lookup("ostemplate").DefValue
	return !cmd.Flags().Changed("ostemplate") && !vp.InConfig("ostemplate") && currentValue == defaultTemplate
}

func mustPromptInt(label string, currentValue int, fallback int) int {
	defaultValue := currentValue
	if defaultValue <= 0 {
		defaultValue = fallback
	}

	for {
		value := promptInput(label, fmt.Sprintf("%d", defaultValue))
		var parsed int
		if _, err := fmt.Sscanf(value, "%d", &parsed); err == nil && parsed > 0 {
			return parsed
		}
		fmt.Println("Please enter a positive integer.")
	}
}

func mustPromptLxcID(auth *proxmox.Auth, node string, currentValue int, fallback int) int {
	defaultValue := currentValue
	if defaultValue <= 0 {
		defaultValue = fallback
	}

	existingLabel := ""
	if strings.TrimSpace(auth.Host) != "" && strings.TrimSpace(auth.ApiToken) != "" && strings.TrimSpace(node) != "" {
		client, err := proxmox.NewClientTokenString(auth.Host, auth.ApiToken, true)
		if err != nil {
			slog.Warn("failed to create proxmox client for LXC ID lookup", "error", err.Error())
		} else {
			containers, err := client.ListLxcContainers(context.Background(), node)
			if err != nil {
				slog.Warn("failed to list existing LXC IDs", "node", node, "error", err.Error())
			} else if len(containers) > 0 {
				ids := make([]string, 0, len(containers))
				for _, container := range containers {
					ids = append(ids, fmt.Sprintf("%d", container.VmId))
				}
				existingLabel = fmt.Sprintf("Existing container IDs on %s: %s", node, strings.Join(ids, ", "))
			} else {
				existingLabel = fmt.Sprintf("Existing container IDs on %s: none", node)
			}
		}
	}

	label := "Container VM ID"
	if existingLabel != "" {
		label = fmt.Sprintf("%s\n%s", label, existingLabel)
	}

	for {
		value := promptInputWithExample(label, "123", fmt.Sprintf("%d", defaultValue))
		var parsed int
		if _, err := fmt.Sscanf(value, "%d", &parsed); err == nil && parsed > 0 {
			return parsed
		}
		fmt.Println("Please enter a positive integer.")
	}
}

func listAvailableLxcTemplates(node string, client *proxmox.Client) ([]string, error) {
	templates, err := client.ListLxcTemplates(context.Background(), node)
	if err == nil && len(templates) > 0 {
		return templates, nil
	}

	slog.Warn("API template lookup failed or returned no templates; trying SSH fallback", "node", node, "error", err)
	templates, sshErr := listAvailableLxcTemplatesOverSSH(node)
	if sshErr != nil {
		if err != nil {
			return nil, fmt.Errorf("api lookup failed: %w; ssh fallback failed: %w", err, sshErr)
		}
		return nil, sshErr
	}
	if len(templates) == 0 {
		if err != nil {
			return nil, fmt.Errorf("api lookup failed: %w; ssh fallback returned no templates", err)
		}
		return nil, fmt.Errorf("no LXC templates found on node %q", node)
	}

	return templates, nil
}

func listAvailableLxcTemplatesOverSSH(node string) ([]string, error) {
	sshHost := rootViperCfg.GetString("ssh_remote_host")
	if strings.TrimSpace(sshHost) == "" {
		sshHost = node
	}

	sshUser := rootViperCfg.GetString("ssh_remote_user")
	if strings.TrimSpace(sshUser) == "" {
		sshUser = "root"
	}

	sshClient, err := infraSSH.InitializeSshClient(
		sshHost,
		sshUser,
		expandPath(rootViperCfg.GetString("ssh_key")),
		rootViperCfg.GetString("ssh_passphrase"),
		rootViperCfg.GetBool("ssh_use_agent"),
		rootViperCfg.GetUint("ssh_port"),
	)
	if err != nil {
		return nil, err
	}
	defer sshClient.Close()

	script := `pvesm status --enabled 1 | awk 'NR>1 {print $1}' | while read -r storage; do pvesm list "$storage" 2>/dev/null | awk 'NR>1 && $1 ~ /:vztmpl\// {print $1}'; done`
	output, err := sshClient.Run("sh -c " + shellQuote(script))
	if err != nil {
		return nil, formatSSHExecError(err, output)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	templates := make([]string, 0, len(lines))
	seen := make(map[string]struct{})
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		templates = append(templates, line)
	}

	return templates, nil
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
	proxmoxLxcCreateCmd.Flags().String("lxc-hostname", "", "Hostname for new lxc container")
	proxmoxLxcCreateCmd.Flags().String("lxc-password", "", "Container root password")
	proxmoxLxcCreateCmd.Flags().String("ostemplate", "local:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst", "OS template")
	proxmoxLxcCreateCmd.Flags().StringSlice("ssh-public-keys", nil, "Authorized SSH public keys")
	proxmoxLxcCreateCmd.Flags().String("storage", "local-lvm", "Storage for container")
	proxmoxLxcCreateCmd.Flags().String("rootfs-size", "9", "Root filesystem size in GB")
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

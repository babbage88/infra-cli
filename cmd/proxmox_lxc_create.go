package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/babbage88/goph/v2"
	"github.com/babbage88/infra-cli/proxmox"
	infraSSH "github.com/babbage88/infra-cli/ssh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	newLxcRequest        proxmox.LxcContainer
	proxmoxLxcAuth       proxmox.Auth
	lxcVerifySSHUserFlag string
	lxcVerifySSHPortFlag uint
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
		if localViper.GetBool("verify") {
			fmt.Println("Verifying container boot and SSH reachability...")
			verifyUser := strings.TrimSpace(localViper.GetString("verify_ssh_user"))
			if verifyUser == "" {
				verifyUser = lxcVerifySSHUserFlag
			}
			verifyPort := localViper.GetUint("verify_ssh_port")
			if verifyPort == 0 {
				verifyPort = lxcVerifySSHPortFlag
			}
			if verifyPort == 0 {
				verifyPort = 22
			}
			if err := verifyCreatedLxcContainer(&newLxcRequest, verifyUser, verifyPort); err != nil {
				return err
			}
		}
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
	if len(req.SshPublicKeys) == 0 {
		req.SshPublicKeys = promptForLxcSSHPublicKeys()
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
	sshClient, _, err := initializeRootSSHClient(node, "root")
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

func promptForLxcSSHPublicKeys() []string {
	if !promptYesNo("Add an SSH public key for container access?", true) {
		return nil
	}

	options := discoverLocalPublicKeyOptions()
	if len(options) == 0 {
		manualKey := strings.TrimSpace(promptInputWithExample("SSH public key", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... you@example.com", ""))
		if manualKey == "" {
			return nil
		}
		return []string{manualKey}
	}

	selectionOptions := append([]string{}, options...)
	selectionOptions = append(selectionOptions, "Paste a public key manually")
	selection := promptSelectOption("Select an SSH public key to authorize", selectionOptions, options[0])
	if selection == "Paste a public key manually" {
		manualKey := strings.TrimSpace(promptInputWithExample("SSH public key", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... you@example.com", ""))
		if manualKey == "" {
			return nil
		}
		return []string{manualKey}
	}

	return []string{selection}
}

func discoverLocalPublicKeyOptions() []string {
	return infraSSH.DiscoverPublicKeyContents(rootViperCfg.GetString("ssh_key"))
}

func verifyCreatedLxcContainer(req *proxmox.LxcContainer, sshUser string, sshPort uint) error {
	sshClient, err := initializeProxmoxAdminSSH(req.Node)
	if err != nil {
		return fmt.Errorf("initialize proxmox SSH for verification: %w", err)
	}
	defer sshClient.Close()

	if req.Start != "1" {
		fmt.Printf("Starting container %d so verification can run...\n", req.VmId)
		if _, err := runRemoteQuotedCommand(sshClient, "pct", "start", fmt.Sprintf("%d", req.VmId)); err != nil {
			return fmt.Errorf("start container %d for verification: %w", req.VmId, err)
		}
	}

	if err := waitForLxcRunningOverSSH(sshClient, req.VmId, 2*time.Minute); err != nil {
		return err
	}

	ipAddr, err := waitForLxcIPv4OverSSH(sshClient, req.VmId, 3*time.Minute)
	if err != nil {
		return err
	}
	fmt.Printf("Container %d reported IPv4 address %s.\n", req.VmId, ipAddr)

	var verificationErrors []string
	if err := verifyLxcSSHFromLocalMachine(ipAddr, sshUser, sshPort); err == nil {
		fmt.Printf("Verified SSH reachability from this computer to %s:%d.\n", ipAddr, sshPort)
		return nil
	} else {
		verificationErrors = append(verificationErrors, fmt.Sprintf("local verification failed: %v", err))
	}

	if err := verifyLxcSSHFromProxmoxNode(sshClient, ipAddr, sshPort); err == nil {
		fmt.Printf("Verified SSH reachability from the Proxmox node to %s:%d.\n", ipAddr, sshPort)
		return nil
	} else {
		verificationErrors = append(verificationErrors, fmt.Sprintf("proxmox-node verification failed: %v", err))
	}

	return fmt.Errorf("container %d booted but SSH was not reachable from either vantage point: %s", req.VmId, strings.Join(verificationErrors, "; "))
}

func waitForLxcRunningOverSSH(sshClient *goph.Client, vmid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		out, err := runRemoteQuotedCommand(sshClient, "pct", "status", fmt.Sprintf("%d", vmid))
		if err == nil && strings.Contains(strings.ToLower(string(out)), "status: running") {
			fmt.Printf("Container %d is running.\n", vmid)
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("timed out waiting for container %d to start: %w", vmid, err)
			}
			return fmt.Errorf("timed out waiting for container %d to start; latest status was %q", vmid, strings.TrimSpace(string(out)))
		}
		time.Sleep(3 * time.Second)
	}
}

func waitForLxcIPv4OverSSH(sshClient *goph.Client, vmid int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		ipAddr, err := getLxcPrimaryIPv4OverSSH(sshClient, vmid)
		if err == nil && ipAddr != "" {
			return ipAddr, nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return "", fmt.Errorf("timed out waiting for IPv4 on container %d: %w", vmid, err)
			}
			return "", fmt.Errorf("timed out waiting for IPv4 on container %d", vmid)
		}
		time.Sleep(5 * time.Second)
	}
}

func getLxcPrimaryIPv4OverSSH(sshClient *goph.Client, vmid int) (string, error) {
	script := `pct exec ` + shellQuote(fmt.Sprintf("%d", vmid)) + ` -- sh -lc ` + shellQuote(`hostname -I 2>/dev/null | tr ' ' '\n' | awk '/^[0-9]+\./ {print $1; exit}'`)
	out, err := sshClient.Run("sh -c " + shellQuote(script))
	if err != nil {
		return "", formatSSHExecError(err, out)
	}

	ipAddr := strings.TrimSpace(string(out))
	if ipAddr == "" {
		return "", fmt.Errorf("container has not reported an IPv4 address yet")
	}
	if net.ParseIP(ipAddr) == nil {
		return "", fmt.Errorf("container reported an invalid IPv4 address %q", ipAddr)
	}
	return ipAddr, nil
}

func verifyLxcSSHFromLocalMachine(ipAddr, sshUser string, sshPort uint) error {
	if strings.TrimSpace(sshUser) == "" {
		sshUser = "root"
	}
	if sshPort == 0 {
		sshPort = 22
	}

	addr := net.JoinHostPort(ipAddr, fmt.Sprintf("%d", sshPort))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("open tcp connection to %s: %w", addr, err)
	}
	_ = conn.Close()

	sshKey := strings.TrimSpace(rootViperCfg.GetString("ssh_key"))
	useAgent := rootViperCfg.GetBool("ssh_use_agent")
	if sshKey == "" && !useAgent {
		return nil
	}

	client, err := infraSSH.InitializeSshClient(
		ipAddr,
		sshUser,
		infraSSH.ExpandPath(sshKey),
		rootViperCfg.GetString("ssh_passphrase"),
		useAgent,
		sshPort,
	)
	if err != nil {
		return fmt.Errorf("authenticate to %s@%s over SSH: %w", sshUser, ipAddr, err)
	}
	defer client.Close()

	out, err := client.Run("true")
	if err != nil {
		return formatSSHExecError(err, out)
	}
	return nil
}

func verifyLxcSSHFromProxmoxNode(sshClient *goph.Client, ipAddr string, sshPort uint) error {
	if sshPort == 0 {
		sshPort = 22
	}
	port := fmt.Sprintf("%d", sshPort)
	script := `if command -v ssh-keyscan >/dev/null 2>&1; then ssh-keyscan -T 5 -p ` + shellQuote(port) + ` ` + shellQuote(ipAddr) + ` >/dev/null 2>&1; else nc -z -w 5 ` + shellQuote(ipAddr) + ` ` + shellQuote(port) + ` >/dev/null 2>&1; fi`
	out, err := sshClient.Run("sh -c " + shellQuote(script))
	if err != nil {
		return formatSSHExecError(err, out)
	}
	return nil
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
	proxmoxLxcCreateCmd.Flags().Bool("verify", false, "Wait for the container to boot and verify SSH reachability after creation")
	proxmoxLxcCreateCmd.Flags().StringVar(&lxcVerifySSHUserFlag, "verify-ssh-user", "root", "SSH user to use when validating container connectivity")
	proxmoxLxcCreateCmd.Flags().UintVar(&lxcVerifySSHPortFlag, "verify-ssh-port", 22, "SSH port to use when validating container connectivity")
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

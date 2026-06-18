package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/babbage88/infra-cli/proxmox"
	infraSSH "github.com/babbage88/infra-cli/ssh"
	"github.com/babbage88/infra-cli/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	newLxcRequest        proxmox.LxcContainer
	proxmoxLxcAuth       proxmox.Auth
	lxcVerifySSHUserFlag string
	lxcVerifySSHPortFlag uint
)

type lxcCreateResultInfo struct {
	GeneratedRootPassword string
	IPv4Address           string
	SSHUser               string
}

type lxcSSHForceOptions = proxmox.LxcSSHForceOptions

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
			OsTemplate:    lxcCreateStringValue(cmd, localViper, "ostemplate"),
			Storage:       lxcCreateStringValue(cmd, localViper, "storage"),
			RootFsSize:    lxcCreateStringValue(cmd, localViper, "rootfs_size"),
			Memory:        lxcCreateIntValue(cmd, localViper, "memory"),
			Swap:          lxcCreateIntValue(cmd, localViper, "swap"),
			Cores:         lxcCreateIntValue(cmd, localViper, "cores"),
			CpuLimit:      lxcCreateIntValue(cmd, localViper, "cpu_limit"),
			CpuUnits:      lxcCreateIntValue(cmd, localViper, "cpu_units"),
			Net0:          lxcCreateStringValue(cmd, localViper, "net0"),
			Arch:          localViper.GetString("arch"),
			Cmode:         localViper.GetString("cmode"),
			Features:      localViper.GetString("features"),
			SshPublicKeys: localViper.GetStringSlice("ssh_public_keys"),
		}
		nestingEnabled := resolveLxcCreateNesting(cmd, localViper)
		if nestingEnabled {
			newLxcRequest.Features = appendProxmoxFeature(newLxcRequest.Features, "nesting=1")
		}
		newLxcRequest.Start = boolToProxmoxFlag(lxcCreateBoolValue(cmd, localViper, "start"))
		newLxcRequest.Console = boolToProxmoxFlag(lxcCreateBoolValue(cmd, localViper, "console"))
		newLxcRequest.Unprivileged = boolToProxmoxFlag(lxcCreateBoolValue(cmd, localViper, "unprivileged"))
		proxmoxLxcAuth.Host = resolveProxmoxAPIHostForNode(cmd, localViper, newLxcRequest.Node, proxmoxLxcAuth.Host)

		resultInfo := lxcCreateResultInfo{}
		if err := promptForMissingLxcCreateBasics(cmd, localViper, &proxmoxLxcAuth, &newLxcRequest, &resultInfo); err != nil {
			return err
		}

		client, err := proxmox.NewClientTokenString(proxmoxLxcAuth.Host, proxmoxLxcAuth.ApiToken, true)
		if err != nil {
			return fmt.Errorf("create proxmox client: %w", err)
		}
		if err := promptForMissingLxcCreateTemplateAndStorage(cmd, localViper, client, &newLxcRequest); err != nil {
			return err
		}
		runInitScript := lxcCreateBoolValue(cmd, localViper, "run_init_script")
		customInitScript := lxcCreateStringValue(cmd, localViper, "custom_script")
		if runInitScript && strings.TrimSpace(customInitScript) == "" {
			customInitScript = promptForLxcCustomInitScript()
		}

		fmt.Println("Creating LXC container...")
		if err := client.CreateLXCContainer(context.Background(), newLxcRequest.Node, &newLxcRequest); err != nil {
			var apiErr *proxmox.APIError
			if nestingEnabled && errors.As(err, &apiErr) && apiErr.Status == http.StatusForbidden {
				return fmt.Errorf("create container with nesting enabled: %w. Proxmox only allows LXC nesting to be set for unprivileged containers by a principal with VM.Allocate on the container path; privileged containers or other feature flags may require root@pam/SuperUser", err)
			}
			return explainLxcCreateError(proxmoxLxcAuth.Host, newLxcRequest.Node, err)
		}
		fmt.Println("Container creation request sent successfully.")
		forceSSH := lxcCreateBoolValue(cmd, localViper, "ssh_force")
		adminOptions := lxcSSHForceOptions{
			AddAdminUser: lxcCreateBoolValue(cmd, localViper, "add_admin_user"),
			AdminUser:    strings.TrimSpace(lxcCreateStringValue(cmd, localViper, "admin_username")),
			AdminUID:     lxcCreateIntValue(cmd, localViper, "admin_uid"),
		}
		if adminOptions.AddAdminUser {
			forceSSH = true
			resultInfo.SSHUser = adminOptions.AdminUser
		}
		if forceSSH || runInitScript {
			ipAddr, err := runLxcPostCreateSetup(&newLxcRequest, adminOptions, forceSSH, runInitScript, customInitScript)
			if err != nil {
				return err
			}
			resultInfo.IPv4Address = ipAddr
		}
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
		printLxcCreateConnectionInfo(&newLxcRequest, resultInfo)
		return nil
	},
}

const defaultLxcCustomInitScript = "#!/usr/bin/env sh\n"

func promptForLxcCustomInitScript() string {
	return tui.TextArea("Custom init script", defaultLxcCustomInitScript)
}

func runLxcCustomInitScriptWithLog(sshClient infraSSH.Client, req *proxmox.LxcContainer, script string, ensureStarted bool, log lxcSSHForceLogSink) error {
	if req == nil {
		return fmt.Errorf("LXC request is required")
	}
	if req.VmId <= 0 {
		return fmt.Errorf("container VM ID is required for --run-init-script")
	}
	if strings.TrimSpace(req.Node) == "" {
		return fmt.Errorf("Proxmox node is required for --run-init-script")
	}
	if strings.TrimSpace(script) == "" {
		return fmt.Errorf("--run-init-script requires --custom-script or a prompted script")
	}

	if ensureStarted && req.Start != "1" {
		lxcSetupStatusf(log, "Starting container %d so the init script can run...", req.VmId)
		if _, err := proxmox.RunRemoteQuotedCommandWithLog(sshClient, log, "pct", "start", fmt.Sprintf("%d", req.VmId)); err != nil {
			return fmt.Errorf("start container %d for --run-init-script: %w", req.VmId, err)
		}
	}
	if err := proxmox.WaitForLxcRunningOverSSHWithLog(sshClient, req.VmId, 2*time.Minute, log); err != nil {
		return fmt.Errorf("wait for container %d to run before --run-init-script: %w", req.VmId, err)
	}

	lxcSetupStatusf(log, "Running custom init script...")
	out, err := proxmox.RunPctExecShellScriptWithLog(sshClient, req.VmId, lxcCustomInitScriptRunner(script), log)
	if log == nil && len(strings.TrimSpace(string(out))) > 0 {
		fmt.Print(string(out))
		if !strings.HasSuffix(string(out), "\n") {
			fmt.Println()
		}
	}
	if err != nil {
		return fmt.Errorf("run custom init script: %w", err)
	}
	lxcSetupStatusf(log, "Custom init script completed successfully.")
	return nil
}

func lxcCustomInitScriptRunner(script string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(script))
	return `set -eu
tmp="$(mktemp /tmp/infractl-init-script.XXXXXX)"
cleanup() { rm -f "$tmp"; }
trap cleanup EXIT
printf %s ` + shellQuote(encoded) + ` | base64 -d > "$tmp"
chmod 700 "$tmp"
"$tmp"`
}

func printLxcCreateConnectionInfo(req *proxmox.LxcContainer, info lxcCreateResultInfo) {
	ipAddr := strings.TrimSpace(info.IPv4Address)
	rootPassword := strings.TrimSpace(info.GeneratedRootPassword)
	sshUser := strings.TrimSpace(info.SSHUser)
	if sshUser == "" {
		sshUser = "root"
	}
	if ipAddr == "" && rootPassword == "" {
		return
	}

	fmt.Println("Container connection information:")
	if req != nil {
		if req.VmId > 0 {
			fmt.Printf("  VMID: %d\n", req.VmId)
		}
		if strings.TrimSpace(req.Hostname) != "" {
			fmt.Printf("  Hostname: %s\n", req.Hostname)
		}
	}
	if ipAddr != "" {
		fmt.Printf("  SSH: ssh %s@%s\n", sshUser, ipAddr)
	}
	if rootPassword != "" {
		fmt.Printf("  Root password: %s\n", rootPassword)
	}
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

func resolveProxmoxAPIHostForNode(cmd *cobra.Command, vp *viper.Viper, node string, currentHost string) string {
	if strings.TrimSpace(currentHost) == "" {
		return defaultProxmoxHostURL(node)
	}
	if cmd != nil && cmd.Flags().Changed("host-url") {
		return currentHost
	}
	if vp != nil && vp.InConfig("host_url") {
		return currentHost
	}
	if derived := defaultProxmoxHostURL(node); strings.TrimSpace(derived) != "" {
		return derived
	}
	return currentHost
}

func resolveLxcCreateNesting(cmd *cobra.Command, vp *viper.Viper) bool {
	if cmd.Flags().Changed("nesting") || vp.InConfig("nesting") {
		return vp.GetBool("nesting")
	}
	if vp.InConfig("proxmox_lxc_defaults_nesting") {
		return vp.GetBool("proxmox_lxc_defaults_nesting")
	}
	if rootViperCfg.IsSet("proxmox_lxc_defaults_nesting") {
		return rootViperCfg.GetBool("proxmox_lxc_defaults_nesting")
	}

	return false
}

func lxcCreateBoolValue(cmd *cobra.Command, vp *viper.Viper, key string) bool {
	flagName := strings.ReplaceAll(key, "_", "-")
	if cmd.Flags().Changed(flagName) || vp.InConfig(key) {
		return vp.GetBool(key)
	}

	defaultKey := "proxmox_lxc_defaults_" + key
	if vp.InConfig(defaultKey) {
		return vp.GetBool(defaultKey)
	}
	if rootViperCfg.IsSet(defaultKey) {
		return rootViperCfg.GetBool(defaultKey)
	}

	flag := cmd.Flags().Lookup(flagName)
	if flag == nil {
		return false
	}

	value, err := strconv.ParseBool(flag.DefValue)
	if err != nil {
		return false
	}
	return value
}

func lxcCreateIntValue(cmd *cobra.Command, vp *viper.Viper, key string) int {
	flagName := strings.ReplaceAll(key, "_", "-")
	if cmd.Flags().Changed(flagName) || vp.InConfig(key) {
		return vp.GetInt(key)
	}

	defaultKey := "proxmox_lxc_defaults_" + key
	if vp.InConfig(defaultKey) {
		return vp.GetInt(defaultKey)
	}
	if rootViperCfg.IsSet(defaultKey) {
		return rootViperCfg.GetInt(defaultKey)
	}

	flag := cmd.Flags().Lookup(flagName)
	if flag == nil {
		return 0
	}

	value, err := strconv.Atoi(flag.DefValue)
	if err != nil {
		return 0
	}
	return value
}

func lxcCreateStringValue(cmd *cobra.Command, vp *viper.Viper, key string) string {
	flagName := strings.ReplaceAll(key, "_", "-")
	if cmd.Flags().Changed(flagName) || vp.InConfig(key) {
		return vp.GetString(key)
	}

	defaultKey := "proxmox_lxc_defaults_" + key
	if vp.InConfig(defaultKey) {
		return vp.GetString(defaultKey)
	}
	if rootViperCfg.IsSet(defaultKey) {
		return rootViperCfg.GetString(defaultKey)
	}

	flag := cmd.Flags().Lookup(flagName)
	if flag == nil {
		return ""
	}
	return flag.DefValue
}

func appendProxmoxFeature(features string, feature string) string {
	features = strings.TrimSpace(features)
	feature = strings.TrimSpace(feature)
	if feature == "" {
		return features
	}
	if features == "" {
		return feature
	}

	for _, existing := range strings.Split(features, ",") {
		if strings.TrimSpace(existing) == feature {
			return features
		}
	}

	return features + "," + feature
}

func boolToProxmoxFlag(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func promptForMissingLxcCreateBasics(cmd *cobra.Command, vp *viper.Viper, auth *proxmox.Auth, req *proxmox.LxcContainer, result *lxcCreateResultInfo) error {
	if strings.TrimSpace(req.Node) == "" {
		req.Node = tui.InputWithExample("Proxmox node", "pve01", "")
	}
	if strings.TrimSpace(auth.Host) == "" {
		defaultHostURL := defaultProxmoxHostURL(req.Node)
		auth.Host = tui.InputWithExample("Proxmox host URL", "https://proxmox.example.com:8006", defaultHostURL)
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
		req.Hostname = tui.InputWithExample("Container hostname", "app-staging-01", "")
	}
	if len(req.SshPublicKeys) == 0 {
		req.SshPublicKeys = promptForLxcSSHPublicKeys()
	}
	if strings.TrimSpace(req.Password) == "" {
		password, usedGenerated, err := promptForLxcRootPasswordWithRandomDefault()
		if err != nil {
			return err
		}
		req.Password = password
		if usedGenerated && result != nil {
			result.GeneratedRootPassword = password
		}
	}

	return nil
}

func promptForLxcRootPasswordWithRandomDefault() (string, bool, error) {
	defaultPassword, err := randomURLPassword(24)
	if err != nil {
		return "", false, fmt.Errorf("generate random root password: %w", err)
	}

	password := tui.PasswordWithExample("Container root password", "press enter to use a generated random password", defaultPassword)
	return password, password == defaultPassword, nil
}

func randomURLPassword(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
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
		if tui.YesNo("Use the configured Proxmox API token and secret?", true) {
			return combinedProxmoxToken(defaults)
		}
		return combinedProxmoxToken(promptForProxmoxTokenParts(defaults))
	}

	return combinedProxmoxToken(promptForProxmoxTokenParts(proxmoxTokenParts{}))
}

func promptForMissingLxcCreateTemplateAndStorage(cmd *cobra.Command, vp *viper.Viper, client *proxmox.Client, req *proxmox.LxcContainer) error {
	if !cmd.Flags().Changed("storage") && !vp.InConfig("storage") && strings.TrimSpace(req.Storage) == "" {
		req.Storage = tui.InputWithExample("Container storage", "local-lvm", "local-lvm")
	} else if strings.TrimSpace(req.Storage) == "" {
		req.Storage = tui.InputWithExample("Container storage", "local-lvm", "local-lvm")
	}
	if !cmd.Flags().Changed("rootfs-size") && !vp.InConfig("rootfs_size") && strings.TrimSpace(req.RootFsSize) == "" {
		req.RootFsSize = tui.InputWithExample("Root filesystem size in GB", "16", "9")
	} else if strings.TrimSpace(req.RootFsSize) == "" {
		req.RootFsSize = tui.InputWithExample("Root filesystem size in GB", "16", "9")
	}
	if shouldPromptForOSTemplate(cmd, vp, req.OsTemplate) {
		templateOptions, err := listAvailableLxcTemplates(req.Node, client)
		if err != nil {
			slog.Warn("failed to list LXC templates automatically; falling back to manual input", "error", err.Error())
			req.OsTemplate = tui.InputWithExample("OS template", "local:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst", req.OsTemplate)
			return nil
		}
		req.OsTemplate = tui.SelectOption("Select OS template", templateOptions, req.OsTemplate)
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
		value := tui.Input(label, fmt.Sprintf("%d", defaultValue))
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
		value := tui.InputWithExample(label, "123", fmt.Sprintf("%d", defaultValue))
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
	command := infraSSH.WithDefaultTERM("sh -c " + shellQuote(script))
	output, err := sshClient.Run(command)
	if err != nil {
		return nil, formatSSHExecError(err, output)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	templates := make([]string, 0, len(lines))
	seen := make(map[string]struct{})
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ":vztmpl/") {
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

func explainLxcCreateError(hostURL, node string, err error) error {
	var apiErr *proxmox.APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnauthorized {
		hostURL = strings.TrimSpace(hostURL)
		if hostURL == "" {
			hostURL = defaultProxmoxHostURL(node)
		}
		return fmt.Errorf(
			"create container on node %s via %s was rejected by the Proxmox API with 401 Unauthorized. This usually means the API token/secret being used for HTTPS requests does not match that Proxmox host, the token was created on a different Proxmox node or cluster endpoint, or the configured host URL is still wrong. Original error: %w",
			node,
			hostURL,
			err,
		)
	}

	return fmt.Errorf("create container: %w", err)
}

func promptForLxcSSHPublicKeys() []string {
	if !tui.YesNo("Add an SSH public key for container access?", true) {
		return nil
	}

	options := discoverLocalPublicKeyOptions()
	if len(options) == 0 {
		manualKey := strings.TrimSpace(tui.InputWithExample("SSH public key", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... you@example.com", ""))
		if manualKey == "" {
			return nil
		}
		return []string{manualKey}
	}

	selected := promptSelectSSHPublicKeys("Select SSH public keys to authorize", options)
	if len(selected) > 0 {
		return selected
	}

	if tui.YesNo("Paste a public key manually?", false) {
		manualKey := strings.TrimSpace(tui.InputWithExample("SSH public key", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... you@example.com", ""))
		if manualKey == "" {
			return nil
		}
		return []string{manualKey}
	}

	return nil
}

func discoverLocalPublicKeyOptions() []infraSSH.PublicKeyOption {
	return infraSSH.DiscoverPublicKeyOptions(rootViperCfg.GetString("ssh_key"))
}

type sshPublicKeySelectModel struct {
	options  []infraSSH.PublicKeyOption
	cursor   int
	selected map[int]struct{}
	done     bool
	cancel   bool
}

func newSSHPublicKeySelectModel(options []infraSSH.PublicKeyOption) sshPublicKeySelectModel {
	selected := make(map[int]struct{})
	if len(options) > 0 {
		selected[0] = struct{}{}
	}
	return sshPublicKeySelectModel{options: options, selected: selected}
}

func (m sshPublicKeySelectModel) Init() tea.Cmd {
	return nil
}

func (m sshPublicKeySelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.Key()
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancel = true
			return m, tea.Quit
		case "q":
			m.cancel = true
			return m, tea.Quit
		case "enter":
			m.done = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		}
		if key.Code == tea.KeySpace || key.Text == " " || msg.String() == "space" {
			m.toggleSelected()
		}
	}
	return m, nil
}

func (m *sshPublicKeySelectModel) toggleSelected() {
	if _, ok := m.selected[m.cursor]; ok {
		delete(m.selected, m.cursor)
		return
	}
	m.selected[m.cursor] = struct{}{}
}

func (m sshPublicKeySelectModel) View() tea.View {
	var builder strings.Builder
	builder.WriteString("Select SSH public keys with space, press enter when done.\n\n")
	for i, option := range m.options {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		check := " "
		if _, ok := m.selected[i]; ok {
			check = "x"
		}
		builder.WriteString(fmt.Sprintf("%s [%s] %s\n", cursor, check, option.Path))
	}
	builder.WriteString("\n")
	return tea.NewView(builder.String())
}

func promptSelectSSHPublicKeys(label string, options []infraSSH.PublicKeyOption) []string {
	if len(options) == 0 {
		return nil
	}
	if tui.IsInteractive() {
		model := newSSHPublicKeySelectModel(options)
		result, err := tea.NewProgram(model).Run()
		if err == nil {
			if selectedModel, ok := result.(sshPublicKeySelectModel); ok && !selectedModel.cancel {
				return selectedSSHPublicKeyContents(options, selectedModel.selected)
			}
		}
	}

	return promptSelectSSHPublicKeysPlain(label, options)
}

func selectedSSHPublicKeyContents(options []infraSSH.PublicKeyOption, selected map[int]struct{}) []string {
	keys := make([]string, 0, len(selected))
	for i, option := range options {
		if _, ok := selected[i]; !ok {
			continue
		}
		keys = append(keys, option.Content)
	}
	return keys
}

func promptSelectSSHPublicKeysPlain(label string, options []infraSSH.PublicKeyOption) []string {
	for i, option := range options {
		fmt.Printf("%d. %s\n", i+1, option.Path)
	}
	for {
		input := strings.TrimSpace(tui.OptionalInput(label+" (comma-separated numbers)", "1"))
		if input == "" {
			return nil
		}
		selected := make(map[int]struct{})
		for _, field := range strings.Split(input, ",") {
			selection, err := strconv.Atoi(strings.TrimSpace(field))
			if err != nil || selection < 1 || selection > len(options) {
				selected = nil
				break
			}
			selected[selection-1] = struct{}{}
		}
		if len(selected) > 0 {
			return selectedSSHPublicKeyContents(options, selected)
		}
		fmt.Printf("Please enter one or more numbers between 1 and %d, separated by commas.\n", len(options))
	}
}

type lxcSSHForceLogSink = proxmox.LxcLogSink
type lxcSSHForceLogKind = proxmox.LxcLogKind
type lxcSSHForceLogEntry = proxmox.LxcLogEntry

const (
	lxcSSHForceLogStatus  = proxmox.LxcLogStatus
	lxcSSHForceLogCommand = proxmox.LxcLogCommand
)

type lxcSSHForceLogMsg lxcSSHForceLogEntry

type lxcSSHForceDoneMsg struct {
	ipAddr string
	err    error
}

type lxcSSHForceViewportModel struct {
	viewport     viewport.Model
	entries      []lxcSSHForceLogEntry
	statuses     []lxcSSHForceLogEntry
	done         bool
	ipAddr       string
	err          error
	contentWidth int
}

func newLxcSSHForceViewportModel() lxcSSHForceViewportModel {
	vp := viewport.New(viewport.WithWidth(104), viewport.WithHeight(16))
	vp.SoftWrap = false
	vp.FillHeight = true
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#18D7FF")).
		Padding(0, 1)

	model := lxcSSHForceViewportModel{viewport: vp, contentWidth: 100}
	model.appendEntry(lxcSSHForceLogEntry{Kind: lxcSSHForceLogStatus, Body: "Preparing LXC post-create setup..."})
	return model
}

func (m lxcSSHForceViewportModel) Init() tea.Cmd {
	return nil
}

func (m lxcSSHForceViewportModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		width := min(112, max(72, msg.Width-8))
		height := min(18, max(8, msg.Height-13))
		m.viewport.SetWidth(width)
		m.viewport.SetHeight(height)
		m.contentWidth = max(32, width-m.viewport.Style.GetHorizontalFrameSize()-2)
		m.renderContent()
	case tea.KeyPressMsg:
		if m.done {
			switch msg.String() {
			case "enter", "q", "esc", "ctrl+c":
				return m, tea.Quit
			}
		}
	case lxcSSHForceLogMsg:
		m.appendEntry(lxcSSHForceLogEntry(msg))
	case lxcSSHForceDoneMsg:
		m.done = true
		m.ipAddr = msg.ipAddr
		m.err = msg.err
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m lxcSSHForceViewportModel) View() tea.View {
	status := lxcSSHForceMutedStyle.Render("running")
	if m.done {
		status = lxcSSHForceSuccessStyle.Render("done")
		if m.err != nil {
			status = lxcSSHForceErrorStyle.Render("failed")
		}
	}

	header := lxcSSHForceTitleStyle.Render("LXC post-create setup") + " " + status + "\n"
	statusPanel := m.statusPanel()
	footer := lxcSSHForceHelpStyle.Render("Scroll: up/down, pgup/pgdn")

	donePrompt := ""
	if m.done {
		doneText := "LXC post-create setup completed successfully. Press enter, q, or esc to continue."
		doneStyle := lxcSSHForceDonePromptStyle
		if m.err != nil {
			doneText = "LXC post-create setup failed: " + lxcSSHForcePromptError(m.err, m.viewport.Width()) + " Press enter, q, or esc to continue."
			doneStyle = lxcSSHForceErrorPromptStyle
		}
		donePrompt = "\n" + doneStyle.Render(doneText)
	}

	outputTitle := lxcSSHForceSectionTitleStyle.Render("stdout / stderr")
	return tea.NewView(header + statusPanel + "\n" + outputTitle + "\n" + m.viewport.View() + "\n" + footer + donePrompt)
}

func (m *lxcSSHForceViewportModel) appendEntry(entry lxcSSHForceLogEntry) {
	entry.Label = tui.CleanTerminalLogText(entry.Label)
	entry.Body = strings.TrimRight(tui.CleanTerminalLogText(entry.Body), "\n")
	m.entries = append(m.entries, entry)
	if entry.Kind == lxcSSHForceLogStatus {
		m.statuses = append(m.statuses, entry)
	}
	m.renderContent()
}

func (m *lxcSSHForceViewportModel) renderContent() {
	m.viewport.SetContent(strings.TrimRight(renderLxcSSHForceCommandEntries(m.entries, m.contentWidth), "\n"))
	m.viewport.GotoBottom()
}

func (m lxcSSHForceViewportModel) statusPanel() string {
	width := max(32, m.viewport.Width())
	style := lxcSSHForceStatusPanelStyle.Width(max(20, width-lxcSSHForceStatusPanelStyle.GetHorizontalFrameSize()))
	bodyWidth := max(20, width-style.GetHorizontalFrameSize())
	statuses := m.statuses
	if len(statuses) > 4 {
		statuses = statuses[len(statuses)-4:]
	}

	var lines []string
	for _, status := range statuses {
		label := "infractl"
		if strings.TrimSpace(status.Label) != "" {
			label = status.Label
		}
		line := label + "  " + status.Body
		lines = append(lines, truncatePlain(line, bodyWidth))
	}
	if len(lines) == 0 {
		lines = append(lines, "waiting for status...")
	}

	for i, line := range lines {
		lines[i] = lxcSSHForceStatusStyle.Render(truncatePlain(line, bodyWidth))
	}

	return lxcSSHForceSectionTitleStyle.Render("status") + "\n" +
		style.Render(strings.Join(lines, "\n")) + "\n"
}

var (
	lxcSSHForceSectionTitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#68717C")).PaddingLeft(1)
	lxcSSHForceTitleStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8A929E"))
	lxcSSHForceMutedStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#5D646F"))
	lxcSSHForceHelpStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F2F6FF"))
	lxcSSHForceSuccessStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#6F9276"))
	lxcSSHForceErrorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#AA7070"))
	lxcSSHForceStatusStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#6C747D"))
	lxcSSHForceStatusLabelStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7F8D8A"))
	lxcSSHForceCommandBlockStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#707883"))
	lxcSSHForceCommandHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#848B96"))
	lxcSSHForceStatusPanelStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#18D7FF")).Padding(0, 1)
	lxcSSHForceDonePromptStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#B8FFE8"))
	lxcSSHForceErrorPromptStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFC2C2"))
)

func renderLxcSSHForceCommandEntries(entries []lxcSSHForceLogEntry, width int) string {
	var builder strings.Builder
	for _, entry := range entries {
		if entry.Kind != lxcSSHForceLogCommand {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(renderLxcSSHForceCommandEntry(entry, width))
	}
	if builder.Len() == 0 {
		builder.WriteString(lxcSSHForceMutedStyle.Render("Waiting for stdout/stderr..."))
		builder.WriteString("\n")
	}
	return builder.String()
}

func renderLxcSSHForceCommandEntry(entry lxcSSHForceLogEntry, width int) string {
	width = max(32, width)
	bodyWidth := max(20, width-2)
	label := "command"
	if strings.TrimSpace(entry.Label) != "" {
		label = tui.CleanTerminalLogText(entry.Label)
	}
	label = truncatePlain(label, bodyWidth)

	var builder strings.Builder
	builder.WriteString(lxcSSHForceCommandHeaderStyle.Render(label))
	builder.WriteString("\n")
	for _, line := range strings.Split(wrapPreformatted(entry.Body, bodyWidth), "\n") {
		builder.WriteString(lxcSSHForceCommandBlockStyle.Render("  " + line))
		builder.WriteString("\n")
	}
	return builder.String()
}

func wrapPreformatted(value string, width int) string {
	lines := strings.Split(strings.TrimRight(tui.CleanTerminalLogText(value), "\n"), "\n")
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			wrapped = append(wrapped, "")
			continue
		}
		for len(line) > width {
			wrapped = append(wrapped, line[:width])
			line = line[width:]
		}
		wrapped = append(wrapped, line)
	}
	return strings.Join(wrapped, "\n")
}

func truncatePlain(value string, width int) string {
	if width <= 0 || len(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:width]
	}
	return value[:width-1] + "…"
}

func lxcSSHForcePromptError(err error, width int) string {
	if err == nil {
		return ""
	}
	value := tui.CleanTerminalLogText(err.Error())
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "unknown error."
	}

	const suffix = " See stdout/stderr above."
	maxWidth := max(48, width-len("LXC post-create setup failed:  Press enter, q, or esc to continue."))
	if len(value)+len(suffix) > maxWidth {
		value = truncatePlain(value, max(16, maxWidth-len(suffix)))
	}
	return value + suffix
}

func padPlainRight(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}

func runLxcPostCreateSetup(req *proxmox.LxcContainer, options lxcSSHForceOptions, forceSSH bool, runInitScript bool, customInitScript string) (string, error) {
	if !tui.IsInteractive() {
		return runLxcPostCreateSetupWithLog(req, options, forceSSH, runInitScript, customInitScript, nil)
	}

	model := newLxcSSHForceViewportModel()
	program := tea.NewProgram(model)
	go func() {
		logger := func(entry lxcSSHForceLogEntry) {
			program.Send(lxcSSHForceLogMsg(entry))
		}
		ipAddr, err := runLxcPostCreateSetupWithLog(req, options, forceSSH, runInitScript, customInitScript, logger)
		program.Send(lxcSSHForceDoneMsg{ipAddr: ipAddr, err: err})
	}()

	result, err := program.Run()
	if err != nil {
		return "", fmt.Errorf("run LXC setup output UI: %w", err)
	}
	finalModel, ok := result.(lxcSSHForceViewportModel)
	if !ok {
		return "", fmt.Errorf("run LXC setup output UI: unexpected model %T", result)
	}
	return finalModel.ipAddr, finalModel.err
}

func runLxcPostCreateSetupWithLog(req *proxmox.LxcContainer, options lxcSSHForceOptions, forceSSH bool, runInitScript bool, customInitScript string, log lxcSSHForceLogSink) (string, error) {
	if req == nil {
		return "", fmt.Errorf("LXC request is required")
	}
	if strings.TrimSpace(req.Node) == "" {
		return "", fmt.Errorf("Proxmox node is required for post-create setup")
	}

	sshClient, err := initializeProxmoxAdminSSH(req.Node)
	if err != nil {
		return "", fmt.Errorf("initialize proxmox SSH for post-create setup: %w", err)
	}
	defer sshClient.Close()

	var ipAddr string
	containerPrepared := false
	if forceSSH {
		lxcSetupStatusf(log, "Forcing container SSH readiness...")
		ipAddr, err = proxmox.ForceLxcSSHReadinessWithLog(sshClient, req, options, log)
		if err != nil {
			return "", err
		}
		containerPrepared = true
	}

	if runInitScript {
		if err := runLxcCustomInitScriptWithLog(sshClient, req, customInitScript, !containerPrepared, log); err != nil {
			return "", err
		}
	}
	if ipAddr == "" {
		ipAddr, err = proxmox.WaitForLxcIPv4OverSSHWithLog(sshClient, req.VmId, 3*time.Minute, log)
		if err != nil {
			return "", err
		}
		lxcSetupStatusf(log, "Container %d reported IPv4 address %s.", req.VmId, ipAddr)
	}

	return ipAddr, nil
}

func lxcSetupStatusf(log lxcSSHForceLogSink, format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	if log == nil {
		fmt.Println(line)
		return
	}
	log(lxcSSHForceLogEntry{Kind: lxcSSHForceLogStatus, Body: line})
}

func forceLxcSSHReadinessWithLog(req *proxmox.LxcContainer, options lxcSSHForceOptions, log lxcSSHForceLogSink) (string, error) {
	if req == nil {
		return "", fmt.Errorf("LXC request is required")
	}
	sshClient, err := initializeProxmoxAdminSSH(req.Node)
	if err != nil {
		return "", fmt.Errorf("initialize proxmox SSH for --ssh-force: %w", err)
	}
	defer sshClient.Close()

	return proxmox.ForceLxcSSHReadinessWithLog(sshClient, req, options, log)
}

func verifyCreatedLxcContainer(req *proxmox.LxcContainer, sshUser string, sshPort uint) error {
	sshClient, err := initializeProxmoxAdminSSH(req.Node)
	if err != nil {
		return fmt.Errorf("initialize proxmox SSH for verification: %w", err)
	}
	defer sshClient.Close()

	if req.Start != "1" {
		fmt.Printf("Starting container %d so verification can run...\n", req.VmId)
		if _, err := proxmox.RunRemoteQuotedCommandWithLog(sshClient, nil, "pct", "start", fmt.Sprintf("%d", req.VmId)); err != nil {
			return fmt.Errorf("start container %d for verification: %w", req.VmId, err)
		}
	}

	if err := proxmox.WaitForLxcRunningOverSSH(sshClient, req.VmId, 2*time.Minute); err != nil {
		return err
	}

	ipAddr, err := proxmox.WaitForLxcIPv4OverSSH(sshClient, req.VmId, 3*time.Minute)
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

	if err := proxmox.VerifyLxcSSHFromProxmoxNode(sshClient, ipAddr, sshPort); err == nil {
		fmt.Printf("Verified SSH reachability from the Proxmox node to %s:%d.\n", ipAddr, sshPort)
		return nil
	} else {
		verificationErrors = append(verificationErrors, fmt.Sprintf("proxmox-node verification failed: %v", err))
	}

	return fmt.Errorf("container %d booted but SSH was not reachable from either vantage point: %s", req.VmId, strings.Join(verificationErrors, "; "))
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
	proxmoxLxcCreateCmd.Flags().Bool("nesting", false, "Enable Proxmox LXC nesting feature")
	proxmoxLxcCreateCmd.Flags().Bool("ssh-force", false, "Install and start SSH inside the container and authorize the selected root SSH key")
	proxmoxLxcCreateCmd.Flags().Bool("add-admin-user", false, "Create a passwordless sudo admin user inside the container")
	proxmoxLxcCreateCmd.Flags().String("admin-username", currentUserName(), "Admin username to create when --add-admin-user is set")
	proxmoxLxcCreateCmd.Flags().Int("admin-uid", 1000, "Admin user UID to create when --add-admin-user is set")
	proxmoxLxcCreateCmd.Flags().Bool("run-init-script", false, "Run a custom init script inside the container after SSH/admin setup")
	proxmoxLxcCreateCmd.Flags().String("custom-script", "", "Custom init script content to run when --run-init-script is set")
	proxmoxLxcCreateCmd.Flags().Bool("start", true, "Start after create")
	proxmoxLxcCreateCmd.Flags().Bool("console", true, "Attach console")
}

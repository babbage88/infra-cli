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
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

type lxcCreateResultInfo struct {
	GeneratedRootPassword string
	IPv4Address           string
}

type lxcSSHForceOptions struct {
	AddAdminUser bool
	AdminUser    string
	AdminUID     int
}

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

		fmt.Println("Creating LXC container...")
		if err := client.CreateLXCContainer(context.Background(), newLxcRequest.Node, &newLxcRequest); err != nil {
			var apiErr *proxmox.APIError
			if nestingEnabled && errors.As(err, &apiErr) && apiErr.Status == http.StatusForbidden {
				return fmt.Errorf("create container with nesting enabled: %w. Proxmox only allows LXC nesting to be set for unprivileged containers by a principal with VM.Allocate on the container path; privileged containers or other feature flags may require root@pam/SuperUser", err)
			}
			return fmt.Errorf("create container: %w", err)
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
		}
		if forceSSH {
			fmt.Println("Forcing container SSH readiness...")
			ipAddr, err := forceLxcSSHReadiness(&newLxcRequest, adminOptions)
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

func printLxcCreateConnectionInfo(req *proxmox.LxcContainer, info lxcCreateResultInfo) {
	if strings.TrimSpace(info.GeneratedRootPassword) == "" {
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
	if strings.TrimSpace(info.IPv4Address) != "" {
		fmt.Printf("  SSH: ssh root@%s\n", info.IPv4Address)
	}
	fmt.Printf("  Root password: %s\n", info.GeneratedRootPassword)
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

	password := promptPasswordWithExample("Container root password", "press enter to use a generated random password", defaultPassword)
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

	selected := promptSelectSSHPublicKeys("Select SSH public keys to authorize", options)
	if len(selected) > 0 {
		return selected
	}

	if promptYesNo("Paste a public key manually?", false) {
		manualKey := strings.TrimSpace(promptInputWithExample("SSH public key", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... you@example.com", ""))
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
	case tea.KeyMsg:
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
		case " ":
			if _, ok := m.selected[m.cursor]; ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		}
	}
	return m, nil
}

func (m sshPublicKeySelectModel) View() string {
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
	return builder.String()
}

func promptSelectSSHPublicKeys(label string, options []infraSSH.PublicKeyOption) []string {
	if len(options) == 0 {
		return nil
	}
	if termIsInteractive() {
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

func termIsInteractive() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
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
		input := strings.TrimSpace(promptOptionalInput(label+" (comma-separated numbers)", "1"))
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

func forceLxcSSHReadiness(req *proxmox.LxcContainer, options lxcSSHForceOptions) (string, error) {
	if req == nil {
		return "", fmt.Errorf("LXC request is required")
	}
	if req.VmId <= 0 {
		return "", fmt.Errorf("container VM ID is required for --ssh-force")
	}
	if strings.TrimSpace(req.Node) == "" {
		return "", fmt.Errorf("Proxmox node is required for --ssh-force")
	}
	if len(req.SshPublicKeys) == 0 {
		return "", fmt.Errorf("--ssh-force requires at least one SSH public key; pass --ssh-public-keys or accept the SSH key prompt")
	}

	sshClient, err := initializeProxmoxAdminSSH(req.Node)
	if err != nil {
		return "", fmt.Errorf("initialize proxmox SSH for --ssh-force: %w", err)
	}
	defer sshClient.Close()

	if req.Start != "1" {
		fmt.Printf("Starting container %d so SSH can be prepared...\n", req.VmId)
		if _, err := runRemoteQuotedCommand(sshClient, "pct", "start", fmt.Sprintf("%d", req.VmId)); err != nil {
			return "", fmt.Errorf("start container %d for --ssh-force: %w", req.VmId, err)
		}
	}

	if err := waitForLxcRunningOverSSH(sshClient, req.VmId, 2*time.Minute); err != nil {
		return "", err
	}

	osInfo, err := detectLxcOSTypeOverSSH(sshClient, req.VmId)
	if err != nil {
		return "", err
	}
	fmt.Printf("Container %d OS: %s.\n", req.VmId, osInfo)

	ipAddr, err := waitForLxcIPv4OverSSH(sshClient, req.VmId, 3*time.Minute)
	if err != nil {
		return "", err
	}
	fmt.Printf("Container %d reported IPv4 address %s.\n", req.VmId, ipAddr)

	if err := ensureLxcSSHServerAndUsersOverSSH(sshClient, req.VmId, req.SshPublicKeys, options); err != nil {
		return "", err
	}
	fmt.Printf("Container %d SSH server is installed, enabled, and has root authorized_keys.\n", req.VmId)
	if options.AddAdminUser {
		fmt.Printf("Container %d admin user %s is ready.\n", req.VmId, options.AdminUser)
	}

	return ipAddr, nil
}

func detectLxcOSTypeOverSSH(sshClient *goph.Client, vmid int) (string, error) {
	script := `if [ -r /etc/os-release ]; then . /etc/os-release; printf '%s' "${PRETTY_NAME:-${ID:-unknown}}"; else uname -s; fi`
	out, err := runPctExecShellScript(sshClient, vmid, script)
	if err != nil {
		return "", fmt.Errorf("detect container OS for %d: %w", vmid, err)
	}

	osInfo := strings.TrimSpace(string(out))
	if osInfo == "" {
		return "unknown", nil
	}
	return osInfo, nil
}

func ensureLxcSSHServerAndRootKeysOverSSH(sshClient *goph.Client, vmid int, publicKeys []string) error {
	keys := sanitizeSSHPublicKeys(publicKeys)
	if len(keys) == 0 {
		return fmt.Errorf("no usable SSH public keys were provided")
	}

	script := `set -eu
if [ -r /etc/os-release ]; then . /etc/os-release; fi
has_sshd() {
	command -v sshd >/dev/null 2>&1 || [ -x /usr/sbin/sshd ] || [ -x /usr/local/sbin/sshd ]
}
if ! has_sshd; then
	if command -v dnf >/dev/null 2>&1; then
		dnf -y install openssh-server
	elif command -v yum >/dev/null 2>&1; then
		yum -y install openssh-server
	elif command -v apt-get >/dev/null 2>&1; then
		apt-get update
		DEBIAN_FRONTEND=noninteractive apt-get install -y openssh-server
	elif command -v zypper >/dev/null 2>&1; then
		zypper --non-interactive install openssh
	elif command -v apk >/dev/null 2>&1; then
		apk add --no-cache openssh
	elif command -v pacman >/dev/null 2>&1; then
		pacman -Sy --noconfirm openssh
	else
		echo "no supported package manager found to install openssh-server" >&2
		exit 1
	fi
fi
install -d -m 0700 /root/.ssh
touch /root/.ssh/authorized_keys
chmod 0600 /root/.ssh/authorized_keys
while IFS= read -r key; do
	[ -n "$key" ] || continue
	grep -qxF "$key" /root/.ssh/authorized_keys || printf '%s\n' "$key" >> /root/.ssh/authorized_keys
done <<'INFRACTL_SSH_KEYS'
` + strings.Join(keys, "\n") + `
INFRACTL_SSH_KEYS
if command -v ssh-keygen >/dev/null 2>&1; then
	ssh-keygen -A
fi
if command -v systemctl >/dev/null 2>&1; then
	systemctl enable --now sshd >/dev/null 2>&1 || systemctl enable --now ssh >/dev/null 2>&1 || true
fi
if ! pgrep -x sshd >/dev/null 2>&1; then
	if command -v service >/dev/null 2>&1; then
		service sshd start >/dev/null 2>&1 || service ssh start >/dev/null 2>&1 || true
	fi
fi
if ! pgrep -x sshd >/dev/null 2>&1; then
	if command -v sshd >/dev/null 2>&1; then
		sshd
	elif [ -x /usr/sbin/sshd ]; then
		/usr/sbin/sshd
	elif [ -x /usr/local/sbin/sshd ]; then
		/usr/local/sbin/sshd
	fi
fi
pgrep -x sshd >/dev/null 2>&1`

	if _, err := runPctExecShellScript(sshClient, vmid, script); err != nil {
		return fmt.Errorf("ensure SSH server and root authorized_keys in container %d: %w", vmid, err)
	}
	return nil
}

func sanitizeSSHPublicKeys(keys []string) []string {
	sanitized := make([]string, 0, len(keys))
	seen := make(map[string]struct{})
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" || strings.ContainsAny(key, "\r\n") {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		sanitized = append(sanitized, key)
	}
	return sanitized
}

func runPctExecShellScript(sshClient *goph.Client, vmid int, script string) ([]byte, error) {
	command := `pct exec ` + shellQuote(fmt.Sprintf("%d", vmid)) + ` -- sh -lc ` + shellQuote(script)
	out, err := sshClient.Run("sh -c " + shellQuote(command))
	if err != nil {
		return nil, formatSSHExecError(err, out)
	}
	return out, nil
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
	out, err := runPctExecShellScript(sshClient, vmid, `hostname -I 2>/dev/null | tr ' ' '\n' | awk '/^[0-9]+\./ {print $1; exit}'`)
	if err != nil {
		return "", err
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
	proxmoxLxcCreateCmd.Flags().Bool("nesting", false, "Enable Proxmox LXC nesting feature")
	proxmoxLxcCreateCmd.Flags().Bool("ssh-force", false, "Install and start SSH inside the container and authorize the selected root SSH key")
	proxmoxLxcCreateCmd.Flags().Bool("start", true, "Start after create")
	proxmoxLxcCreateCmd.Flags().Bool("console", true, "Attach console")
}

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
	"github.com/babbage88/goph/v2"
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
			ipAddr, err := runLxcSSHForceReadiness(&newLxcRequest, adminOptions)
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

type lxcSSHForceLogSink func(lxcSSHForceLogEntry)

type lxcSSHForceLogKind string

const (
	lxcSSHForceLogStatus  lxcSSHForceLogKind = "status"
	lxcSSHForceLogCommand lxcSSHForceLogKind = "command"
)

type lxcSSHForceLogEntry struct {
	Kind  lxcSSHForceLogKind
	Label string
	Body  string
}

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
		BorderForeground(lipgloss.Color("#343A43")).
		Padding(0, 1)

	model := lxcSSHForceViewportModel{viewport: vp, contentWidth: 100}
	model.appendEntry(lxcSSHForceLogEntry{Kind: lxcSSHForceLogStatus, Body: "Forcing container SSH readiness..."})
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
		if msg.err != nil {
			m.appendEntry(lxcSSHForceLogEntry{Kind: lxcSSHForceLogStatus, Label: "failed", Body: msg.err.Error()})
		} else {
			m.appendEntry(lxcSSHForceLogEntry{Kind: lxcSSHForceLogStatus, Label: "done", Body: "SSH readiness completed."})
		}
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

	header := lxcSSHForceTitleStyle.Render("LXC SSH force setup") + " " + status + "\n"
	statusPanel := m.statusPanel()
	footer := lxcSSHForceHelpStyle.Render("Scroll: up/down, pgup/pgdn")
	if m.done {
		footer += lxcSSHForceHelpStyle.Render("  Close: enter/q/esc")
	}

	outputTitle := lxcSSHForceSectionTitleStyle.Render("stdout / stderr")
	return tea.NewView(header + statusPanel + "\n" + outputTitle + "\n" + m.viewport.View() + "\n" + footer)
}

func (m *lxcSSHForceViewportModel) appendEntry(entry lxcSSHForceLogEntry) {
	entry.Body = strings.TrimRight(entry.Body, "\n")
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
	bodyWidth := max(20, width-2)
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
		lxcSSHForceStatusPanelStyle.Width(width).Render(strings.Join(lines, "\n")) + "\n"
}

var (
	lxcSSHForceSectionTitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#68717C")).PaddingLeft(1)
	lxcSSHForceTitleStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8A929E"))
	lxcSSHForceMutedStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#5D646F"))
	lxcSSHForceHelpStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#4E5560"))
	lxcSSHForceSuccessStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#6F9276"))
	lxcSSHForceErrorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#AA7070"))
	lxcSSHForceStatusStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#6C747D"))
	lxcSSHForceStatusLabelStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7F8D8A"))
	lxcSSHForceCommandBlockStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#707883"))
	lxcSSHForceCommandHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#848B96"))
	lxcSSHForceStatusPanelStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#343A43")).Padding(0, 1)
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
		label = entry.Label
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
	lines := strings.Split(strings.TrimRight(value, "\n"), "\n")
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

func padPlainRight(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}

func runLxcSSHForceReadiness(req *proxmox.LxcContainer, options lxcSSHForceOptions) (string, error) {
	if !tui.IsInteractive() {
		fmt.Println("Forcing container SSH readiness...")
		return forceLxcSSHReadiness(req, options)
	}

	model := newLxcSSHForceViewportModel()
	program := tea.NewProgram(model)
	go func() {
		logger := func(entry lxcSSHForceLogEntry) {
			program.Send(lxcSSHForceLogMsg(entry))
		}
		ipAddr, err := forceLxcSSHReadinessWithLog(req, options, logger)
		program.Send(lxcSSHForceDoneMsg{ipAddr: ipAddr, err: err})
	}()

	result, err := program.Run()
	if err != nil {
		return "", fmt.Errorf("run SSH force output UI: %w", err)
	}
	finalModel, ok := result.(lxcSSHForceViewportModel)
	if !ok {
		return "", fmt.Errorf("run SSH force output UI: unexpected model %T", result)
	}
	return finalModel.ipAddr, finalModel.err
}

func forceLxcSSHReadiness(req *proxmox.LxcContainer, options lxcSSHForceOptions) (string, error) {
	return forceLxcSSHReadinessWithLog(req, options, nil)
}

func forceLxcSSHReadinessWithLog(req *proxmox.LxcContainer, options lxcSSHForceOptions, log lxcSSHForceLogSink) (string, error) {
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
		lxcSSHForceStatusf(log, "Starting container %d so SSH can be prepared...", req.VmId)
		if _, err := runRemoteQuotedCommandWithLog(sshClient, log, "pct", "start", fmt.Sprintf("%d", req.VmId)); err != nil {
			return "", fmt.Errorf("start container %d for --ssh-force: %w", req.VmId, err)
		}
	}

	if err := waitForLxcRunningOverSSHWithLog(sshClient, req.VmId, 2*time.Minute, log); err != nil {
		return "", err
	}

	osInfo, err := detectLxcOSTypeOverSSHWithLog(sshClient, req.VmId, log)
	if err != nil {
		return "", err
	}
	lxcSSHForceStatusf(log, "Container %d OS: %s.", req.VmId, osInfo)

	ipAddr, err := waitForLxcIPv4OverSSHWithLog(sshClient, req.VmId, 3*time.Minute, log)
	if err != nil {
		return "", err
	}
	lxcSSHForceStatusf(log, "Container %d reported IPv4 address %s.", req.VmId, ipAddr)

	if err := ensureLxcSSHServerAndUsersOverSSHWithLog(sshClient, req.VmId, req.SshPublicKeys, options, log); err != nil {
		return "", err
	}
	lxcSSHForceStatusf(log, "Container %d SSH server is installed, enabled, and has root authorized_keys.", req.VmId)
	if options.AddAdminUser {
		lxcSSHForceStatusf(log, "Container %d admin user %s is ready.", req.VmId, options.AdminUser)
	}

	return ipAddr, nil
}

func detectLxcOSTypeOverSSH(sshClient *goph.Client, vmid int) (string, error) {
	return detectLxcOSTypeOverSSHWithLog(sshClient, vmid, nil)
}

func detectLxcOSTypeOverSSHWithLog(sshClient *goph.Client, vmid int, log lxcSSHForceLogSink) (string, error) {
	script := `if [ -r /etc/os-release ]; then . /etc/os-release; printf '%s' "${PRETTY_NAME:-${ID:-unknown}}"; else uname -s; fi`
	out, err := runPctExecShellScriptWithLog(sshClient, vmid, script, log)
	if err != nil {
		return "", fmt.Errorf("detect container OS for %d: %w", vmid, err)
	}

	osInfo := strings.TrimSpace(string(out))
	if osInfo == "" {
		return "unknown", nil
	}
	return osInfo, nil
}

func ensureLxcSSHServerAndUsersOverSSH(sshClient *goph.Client, vmid int, publicKeys []string, options lxcSSHForceOptions) error {
	return ensureLxcSSHServerAndUsersOverSSHWithLog(sshClient, vmid, publicKeys, options, nil)
}

func ensureLxcSSHServerAndUsersOverSSHWithLog(sshClient *goph.Client, vmid int, publicKeys []string, options lxcSSHForceOptions, log lxcSSHForceLogSink) error {
	keys := sanitizeSSHPublicKeys(publicKeys)
	if len(keys) == 0 {
		return fmt.Errorf("no usable SSH public keys were provided")
	}
	if options.AddAdminUser {
		options.AdminUser = strings.TrimSpace(options.AdminUser)
		if options.AdminUser == "" {
			return fmt.Errorf("--admin-username is required when --add-admin-user is set")
		}
		if strings.ContainsAny(options.AdminUser, "\r\n:/'\"`$\\ ") {
			return fmt.Errorf("admin username %q contains unsupported characters", options.AdminUser)
		}
		if options.AdminUID <= 0 {
			return fmt.Errorf("--admin-uid must be greater than zero when --add-admin-user is set")
		}
	}

	lxcSSHForceStatusf(log, "Ensuring SSH server, authorized_keys, and requested users inside container %d...", vmid)

	script := `set -eu
if [ -r /etc/os-release ]; then . /etc/os-release; fi
has_sshd() {
	command -v sshd >/dev/null 2>&1 || [ -x /usr/sbin/sshd ] || [ -x /usr/local/sbin/sshd ]
}
install_pkg() {
	if command -v dnf >/dev/null 2>&1; then
		dnf -y install "$@"
	elif command -v yum >/dev/null 2>&1; then
		yum -y install "$@"
	elif command -v apt-get >/dev/null 2>&1; then
		apt-get update
		DEBIAN_FRONTEND=noninteractive apt-get install -y "$@"
	elif command -v zypper >/dev/null 2>&1; then
		zypper --non-interactive install "$@"
	elif command -v apk >/dev/null 2>&1; then
		apk add --no-cache "$@"
	elif command -v pacman >/dev/null 2>&1; then
		pacman -Sy --noconfirm "$@"
	else
		echo "no supported package manager found" >&2
		exit 1
	fi
}
if ! has_sshd; then
	if command -v dnf >/dev/null 2>&1; then
		install_pkg openssh-server
	elif command -v yum >/dev/null 2>&1; then
		install_pkg openssh-server
	elif command -v apt-get >/dev/null 2>&1; then
		install_pkg openssh-server
	elif command -v zypper >/dev/null 2>&1; then
		install_pkg openssh
	elif command -v apk >/dev/null 2>&1; then
		install_pkg openssh
	elif command -v pacman >/dev/null 2>&1; then
		install_pkg openssh
	else
		echo "no supported package manager found to install openssh-server" >&2
		exit 1
	fi
fi
ensure_authorized_keys() {
	user_name="$1"
	user_home="$2"
	install -d -m 0700 "$user_home/.ssh"
	touch "$user_home/.ssh/authorized_keys"
	chmod 0600 "$user_home/.ssh/authorized_keys"
	while IFS= read -r key; do
		[ -n "$key" ] || continue
		grep -qxF "$key" "$user_home/.ssh/authorized_keys" || printf '%s\n' "$key" >> "$user_home/.ssh/authorized_keys"
	done <<'INFRACTL_SSH_KEYS'
` + strings.Join(keys, "\n") + `
INFRACTL_SSH_KEYS
	if [ "$user_name" != "root" ]; then
		chown -R "$user_name:$user_name" "$user_home/.ssh" 2>/dev/null || chown -R "$user_name" "$user_home/.ssh"
	fi
}
ensure_authorized_keys root /root
` + adminUserBootstrapScript(options) + `
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

	if _, err := runPctExecShellScriptWithLog(sshClient, vmid, script, log); err != nil {
		return fmt.Errorf("ensure SSH server, users, and authorized_keys in container %d: %w", vmid, err)
	}
	return nil
}

func adminUserBootstrapScript(options lxcSSHForceOptions) string {
	if !options.AddAdminUser {
		return ""
	}

	adminUser := shellQuote(options.AdminUser)
	adminUID := shellQuote(fmt.Sprintf("%d", options.AdminUID))
	sudoersPath := shellQuote("/etc/sudoers.d/90-infractl-" + options.AdminUser)

	return `
if ! command -v sudo >/dev/null 2>&1; then
	install_pkg sudo
fi
admin_user=` + adminUser + `
admin_uid=` + adminUID + `
user_home_from_passwd() {
	awk -F: -v user="$1" '$1 == user {print $6; exit}' /etc/passwd
}
if id "$admin_user" >/dev/null 2>&1; then
	admin_home=$(user_home_from_passwd "$admin_user")
else
	admin_shell=/bin/sh
	[ -x /bin/bash ] && admin_shell=/bin/bash
	if command -v useradd >/dev/null 2>&1; then
		useradd -m -u "$admin_uid" -s "$admin_shell" "$admin_user"
	elif command -v adduser >/dev/null 2>&1; then
		adduser -D -u "$admin_uid" -s "$admin_shell" "$admin_user"
	else
		echo "no supported user creation command found" >&2
		exit 1
	fi
	admin_home=$(user_home_from_passwd "$admin_user")
fi
[ -n "$admin_home" ] || admin_home="/home/$admin_user"
install -d -m 0755 /etc/sudoers.d
printf '%s ALL=(ALL) NOPASSWD:ALL\n' "$admin_user" > ` + sudoersPath + `
chmod 0440 ` + sudoersPath + `
ensure_authorized_keys "$admin_user" "$admin_home"
`
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
	return runPctExecShellScriptWithLog(sshClient, vmid, script, nil)
}

func runPctExecShellScriptWithLog(sshClient *goph.Client, vmid int, script string, log lxcSSHForceLogSink) ([]byte, error) {
	command := `pct exec ` + shellQuote(fmt.Sprintf("%d", vmid)) + ` -- sh -lc ` + shellQuote(script)
	out, err := sshClient.Run("sh -c " + shellQuote(command))
	lxcSSHForceCommandOutput(log, fmt.Sprintf("pct exec %d", vmid), out)
	if err != nil {
		return nil, formatSSHExecError(err, out)
	}
	return out, nil
}

func runRemoteQuotedCommandWithLog(sshClient *goph.Client, log lxcSSHForceLogSink, args ...string) ([]byte, error) {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}

	command := strings.Join(quoted, " ")
	out, err := sshClient.Run(command)
	lxcSSHForceCommandOutput(log, strings.Join(args, " "), out)
	if err != nil {
		return nil, formatSSHExecError(err, out)
	}

	return out, nil
}

func lxcSSHForceStatusf(log lxcSSHForceLogSink, format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	if log == nil {
		fmt.Println(line)
		return
	}
	log(lxcSSHForceLogEntry{Kind: lxcSSHForceLogStatus, Body: line})
}

func lxcSSHForceCommandOutput(log lxcSSHForceLogSink, label string, out []byte) {
	if log == nil {
		return
	}
	if strings.HasPrefix(label, "pct status ") {
		return
	}
	output := strings.TrimSpace(string(out))
	if output == "" {
		return
	}
	log(lxcSSHForceLogEntry{Kind: lxcSSHForceLogCommand, Label: label, Body: output})
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
	return waitForLxcRunningOverSSHWithLog(sshClient, vmid, timeout, nil)
}

func waitForLxcRunningOverSSHWithLog(sshClient *goph.Client, vmid int, timeout time.Duration, log lxcSSHForceLogSink) error {
	deadline := time.Now().Add(timeout)
	lxcSSHForceStatusf(log, "Waiting for container %d to report running status...", vmid)
	for {
		out, err := runRemoteQuotedCommandWithLog(sshClient, log, "pct", "status", fmt.Sprintf("%d", vmid))
		if err == nil && strings.Contains(strings.ToLower(string(out)), "status: running") {
			lxcSSHForceStatusf(log, "Container %d is running.", vmid)
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
	return waitForLxcIPv4OverSSHWithLog(sshClient, vmid, timeout, nil)
}

func waitForLxcIPv4OverSSHWithLog(sshClient *goph.Client, vmid int, timeout time.Duration, log lxcSSHForceLogSink) (string, error) {
	deadline := time.Now().Add(timeout)
	lxcSSHForceStatusf(log, "Waiting for container %d to report an IPv4 address...", vmid)
	for {
		ipAddr, err := getLxcPrimaryIPv4OverSSHWithLog(sshClient, vmid, log)
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
	return getLxcPrimaryIPv4OverSSHWithLog(sshClient, vmid, nil)
}

func getLxcPrimaryIPv4OverSSHWithLog(sshClient *goph.Client, vmid int, log lxcSSHForceLogSink) (string, error) {
	out, err := runPctExecShellScriptWithLog(sshClient, vmid, `hostname -I 2>/dev/null | tr ' ' '\n' | awk '/^[0-9]+\./ {print $1; exit}'`, log)
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
	proxmoxLxcCreateCmd.Flags().Bool("add-admin-user", false, "Create a passwordless sudo admin user inside the container")
	proxmoxLxcCreateCmd.Flags().String("admin-username", currentUserName(), "Admin username to create when --add-admin-user is set")
	proxmoxLxcCreateCmd.Flags().Int("admin-uid", 1000, "Admin user UID to create when --add-admin-user is set")
	proxmoxLxcCreateCmd.Flags().Bool("start", true, "Start after create")
	proxmoxLxcCreateCmd.Flags().Bool("console", true, "Attach console")
}

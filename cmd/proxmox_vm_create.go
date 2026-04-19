package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/babbage88/infra-cli/proxmox"
	"github.com/babbage88/infra-cli/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	createVmID int
)

type vmCreateRequest struct {
	Node              string
	TemplateVMID      int
	Name              string
	MemoryMB          int
	Sockets           int
	Cores             int
	Description       string
	Storage           string
	FullClone         bool
	Start             bool
	CIUser            string
	CIPassword        string
	SSHPublicKeys     []string
	IPConfig0         string
	Nameserver        string
	SearchDomain      string
	CISnippetsStorage string
	CICustomScript    string
}

type vmTemplateOption struct {
	VMID  int
	Label string
}

var proxmoxVmCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"new", "add"},
	Short:   "Create a new Proxmox VM from a VM template",
	RunE: func(cmd *cobra.Command, args []string) error {
		localViper := viper.New()

		cfgFile, _ := cmd.Flags().GetString("config-file")
		if cfgFile != "" {
			if err := loadProxmoxConfigFile(cfgFile, localViper); err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
		}
		bindLocalFlags(cmd, localViper)
		applyRootProxmoxDefaults(localViper)

		auth := proxmox.Auth{
			Host:     localViper.GetString("host_url"),
			ApiToken: localViper.GetString("api_token"),
		}
		req := vmCreateRequest{
			Node:              vmCreateStringValue(cmd, localViper, "pve_node"),
			TemplateVMID:      vmCreateIntValue(cmd, localViper, "template_vmid"),
			Name:              vmCreateStringValue(cmd, localViper, "name"),
			MemoryMB:          vmCreateIntValue(cmd, localViper, "memory"),
			Sockets:           vmCreateIntValue(cmd, localViper, "sockets"),
			Cores:             vmCreateIntValue(cmd, localViper, "cores"),
			Description:       vmCreateStringValue(cmd, localViper, "description"),
			Storage:           vmCreateStringValue(cmd, localViper, "storage"),
			FullClone:         vmCreateBoolValue(cmd, localViper, "full_clone"),
			Start:             vmCreateBoolValue(cmd, localViper, "start"),
			CIUser:            vmCreateStringValue(cmd, localViper, "ci_user"),
			CIPassword:        vmCreateStringValue(cmd, localViper, "ci_password"),
			SSHPublicKeys:     localViper.GetStringSlice("ssh_public_keys"),
			IPConfig0:         vmCreateStringValue(cmd, localViper, "ipconfig0"),
			Nameserver:        vmCreateStringValue(cmd, localViper, "nameserver"),
			SearchDomain:      vmCreateStringValue(cmd, localViper, "searchdomain"),
			CISnippetsStorage: vmCreateStringValue(cmd, localViper, "ci_snippets_storage"),
			CICustomScript:    vmCreateStringValue(cmd, localViper, "ci_custom_script"),
		}
		if strings.TrimSpace(req.CICustomScript) == "" {
			req.CICustomScript = vmCreateStringValue(cmd, localViper, "ci_custum_script")
		}
		if createVmID != 0 {
			reqID := localViper.GetInt("vmid")
			if reqID == 0 {
				reqID = createVmID
			}
			createVmID = reqID
		}

		if err := promptForMissingVMCreateBasics(cmd, localViper, &auth, &req); err != nil {
			return err
		}

		client, err := proxmox.NewClientTokenString(auth.Host, auth.ApiToken, true)
		if err != nil {
			return fmt.Errorf("create proxmox client: %w", err)
		}
		if err := promptForMissingVMTemplate(cmd, localViper, client, &req); err != nil {
			return err
		}
		if err := promptForMissingVMCloudInit(cmd, localViper, &req); err != nil {
			return err
		}

		cloudInitConfig, err := buildVMCloudInitConfig(&req)
		if err != nil {
			return err
		}
		if strings.TrimSpace(req.CICustomScript) != "" {
			volid, err := uploadVMCloudInitCustomScript(req.Node, createVmID, req.CISnippetsStorage, req.CICustomScript)
			if err != nil {
				return err
			}
			if cloudInitConfig.Raw == nil {
				cloudInitConfig.Raw = make(map[string]string)
			}
			cloudInitConfig.Raw["cicustom"] = "user=" + volid
		}

		fmt.Println("Cloning VM template...")
		cloneReq := proxmox.QemuCloneRequest{
			NewID:       createVmID,
			Name:        req.Name,
			Storage:     req.Storage,
			Description: req.Description,
			FullClone:   req.FullClone,
		}
		if err := client.CloneVM(context.Background(), req.Node, req.TemplateVMID, cloneReq); err != nil {
			return fmt.Errorf("clone VM template %d: %w", req.TemplateVMID, err)
		}
		fmt.Println("VM clone request sent successfully.")
		if err := waitForVMUnlocked(context.Background(), client, req.Node, createVmID, 5*time.Minute); err != nil {
			return err
		}

		if hasVMConfigValues(cloudInitConfig) {
			fmt.Println("Applying VM and cloud-init configuration...")
			if err := client.UpdateVMConfig(context.Background(), req.Node, createVmID, cloudInitConfig); err != nil {
				return fmt.Errorf("apply VM cloud-init config: %w", err)
			}
		}

		if req.Start {
			fmt.Println("Starting VM...")
			if _, err := client.StartVM(context.Background(), req.Node, createVmID); err != nil {
				return fmt.Errorf("start VM %d: %w", createVmID, err)
			}
		}

		printVMCreateConnectionInfo(&req, createVmID)
		return nil
	},
}

const defaultVMCustomInitScript = "#!/usr/bin/env sh\n"

func promptForMissingVMCreateBasics(cmd *cobra.Command, vp *viper.Viper, auth *proxmox.Auth, req *vmCreateRequest) error {
	if strings.TrimSpace(req.Node) == "" {
		req.Node = tui.InputWithExample("Proxmox node", "pve01", "")
	}
	if strings.TrimSpace(auth.Host) == "" {
		auth.Host = tui.InputWithExample("Proxmox host URL", "https://proxmox.example.com:8006", defaultProxmoxHostURL(req.Node))
	}
	if shouldPromptForProxmoxAPIAuth(cmd, vp, auth.ApiToken) {
		auth.ApiToken = promptForProxmoxAPIAuthToken()
	}
	if !cmd.Flags().Changed("vmid") && !vp.InConfig("vmid") {
		createVmID = mustPromptVMID(auth, req.Node, createVmID, 9000)
	} else if createVmID <= 0 {
		createVmID = mustPromptVMID(auth, req.Node, createVmID, 9000)
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = tui.InputWithExample("VM name", "app-staging-01", "")
	}
	return nil
}

func promptForMissingVMTemplate(cmd *cobra.Command, vp *viper.Viper, client *proxmox.Client, req *vmCreateRequest) error {
	if req.TemplateVMID > 0 {
		return nil
	}

	templates, err := listAvailableVMTemplates(req.Node, client)
	if err != nil {
		slog.Warn("failed to list VM templates automatically; falling back to manual input", "error", err.Error())
		req.TemplateVMID = mustPromptInt("Template VM ID", req.TemplateVMID, 9000)
		return nil
	}
	if len(templates) == 0 {
		req.TemplateVMID = mustPromptInt("Template VM ID", req.TemplateVMID, 9000)
		return nil
	}

	labels := make([]string, 0, len(templates))
	idByLabel := make(map[string]int, len(templates))
	for _, option := range templates {
		labels = append(labels, option.Label)
		idByLabel[option.Label] = option.VMID
	}

	selected := tui.SelectOption("Select VM template", labels, "")
	req.TemplateVMID = idByLabel[selected]
	if req.TemplateVMID <= 0 {
		req.TemplateVMID = mustPromptInt("Template VM ID", req.TemplateVMID, templates[0].VMID)
	}
	return nil
}

func promptForMissingVMCloudInit(cmd *cobra.Command, vp *viper.Viper, req *vmCreateRequest) error {
	if len(req.SSHPublicKeys) == 0 {
		req.SSHPublicKeys = promptForLxcSSHPublicKeys()
	}
	if len(req.SSHPublicKeys) == 0 && strings.TrimSpace(req.CIPassword) == "" && tui.YesNo("Set a cloud-init password?", false) {
		req.CIPassword = tui.PasswordWithExample("Cloud-init password", "password for the cloud-init user", "")
	}
	if strings.TrimSpace(req.CIUser) == "" {
		req.CIUser = tui.InputWithExample("Cloud-init user", "admin", currentUserName())
	}
	if strings.TrimSpace(req.IPConfig0) == "" {
		req.IPConfig0 = tui.InputWithExample("Cloud-init ipconfig0", "ip=dhcp", "ip=dhcp")
	}
	if shouldPromptForVMCustomScript(cmd, vp, req.CICustomScript) && tui.YesNo("Add a custom cloud-init script?", false) {
		req.CICustomScript = tui.TextArea("Custom cloud-init script", defaultVMCustomInitScript)
	}
	if strings.TrimSpace(req.CICustomScript) != "" && strings.TrimSpace(req.CISnippetsStorage) == "" {
		req.CISnippetsStorage = tui.InputWithExample("Cloud-init snippets storage", "local", "local")
	}
	return nil
}

func shouldPromptForVMCustomScript(cmd *cobra.Command, vp *viper.Viper, currentValue string) bool {
	if strings.TrimSpace(currentValue) != "" {
		return false
	}
	return !cmd.Flags().Changed("ci-custom-script") && !cmd.Flags().Changed("ci-custum-script") && !vp.InConfig("ci_custom_script") && !vp.InConfig("ci_custum_script")
}

func buildVMCloudInitConfig(req *vmCreateRequest) (*proxmox.ProxmoxQemuVmConfig, error) {
	cfg := &proxmox.ProxmoxQemuVmConfig{
		Name:        req.Name,
		MemoryMB:    intToJsonNumber(req.MemoryMB),
		Sockets:     intToJsonNumber(req.Sockets),
		Cores:       intToJsonNumber(req.Cores),
		Description: req.Description,
		Raw:         make(map[string]string),
	}
	if req.MemoryMB <= 0 {
		cfg.MemoryMB = ""
	}
	if req.Sockets <= 0 {
		cfg.Sockets = ""
	}
	if req.Cores <= 0 {
		cfg.Cores = ""
	}
	if strings.TrimSpace(req.CIUser) != "" {
		cfg.Raw["ciuser"] = req.CIUser
	}
	if strings.TrimSpace(req.CIPassword) != "" {
		cfg.Raw["cipassword"] = req.CIPassword
	}
	if len(req.SSHPublicKeys) > 0 {
		keys, err := joinSSHPublicKeys(req.SSHPublicKeys)
		if err != nil {
			return nil, err
		}
		cfg.Raw["sshkeys"] = keys
	}
	if strings.TrimSpace(req.IPConfig0) != "" {
		cfg.Raw["ipconfig0"] = req.IPConfig0
	}
	if strings.TrimSpace(req.Nameserver) != "" {
		cfg.Raw["nameserver"] = req.Nameserver
	}
	if strings.TrimSpace(req.SearchDomain) != "" {
		cfg.Raw["searchdomain"] = req.SearchDomain
	}
	if len(cfg.Raw) == 0 {
		cfg.Raw = nil
	}
	return cfg, nil
}

func hasVMConfigValues(cfg *proxmox.ProxmoxQemuVmConfig) bool {
	return cfg != nil &&
		(cfg.Name != "" ||
			cfg.MemoryMB != "" ||
			cfg.Sockets != "" ||
			cfg.Cores != "" ||
			cfg.Description != "" ||
			len(cfg.Raw) > 0)
}

func waitForVMUnlocked(ctx context.Context, client *proxmox.Client, node string, vmid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		cfg, err := client.GetVMConfig(ctx, node, vmid)
		if err == nil {
			if cfg.Raw == nil || strings.TrimSpace(cfg.Raw["lock"]) == "" {
				return nil
			}
			lastErr = fmt.Errorf("VM %d is locked: %s", vmid, cfg.Raw["lock"])
		} else {
			lastErr = err
		}

		if time.Now().After(deadline) {
			if lastErr != nil {
				return fmt.Errorf("timed out waiting for VM %d clone to finish: %w", vmid, lastErr)
			}
			return fmt.Errorf("timed out waiting for VM %d clone to finish", vmid)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func joinSSHPublicKeys(keys []string) (string, error) {
	cleaned := make([]string, 0, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key != "" {
			cleaned = append(cleaned, key)
		}
	}
	if len(cleaned) == 0 {
		return "", fmt.Errorf("empty slice: no ssh keys provided")
	}
	return strings.Join(cleaned, "\n"), nil
}

func listAvailableVMTemplates(node string, client *proxmox.Client) ([]vmTemplateOption, error) {
	vms, err := client.ListVMs(context.Background(), node, true)
	if err != nil {
		return nil, err
	}

	options := make([]vmTemplateOption, 0)
	for _, vm := range vms {
		if vm.Template == 0 {
			continue
		}
		name := strings.TrimSpace(vm.Name)
		if name == "" {
			name = "(unnamed)"
		}
		options = append(options, vmTemplateOption{
			VMID:  vm.Vmid,
			Label: fmt.Sprintf("%d - %s", vm.Vmid, name),
		})
	}
	sort.Slice(options, func(i, j int) bool {
		return options[i].VMID < options[j].VMID
	})
	return options, nil
}

func mustPromptVMID(auth *proxmox.Auth, node string, currentValue int, fallback int) int {
	defaultValue := currentValue
	if defaultValue <= 0 {
		defaultValue = fallback
	}

	existingLabel := ""
	if strings.TrimSpace(auth.Host) != "" && strings.TrimSpace(auth.ApiToken) != "" && strings.TrimSpace(node) != "" {
		client, err := proxmox.NewClientTokenString(auth.Host, auth.ApiToken, true)
		if err != nil {
			slog.Warn("failed to create proxmox client for VM ID lookup", "error", err.Error())
		} else {
			vms, err := client.ListVMs(context.Background(), node, false)
			if err != nil {
				slog.Warn("failed to list existing VM IDs", "node", node, "error", err.Error())
			} else if len(vms) > 0 {
				ids := make([]string, 0, len(vms))
				for _, vm := range vms {
					ids = append(ids, fmt.Sprintf("%d", vm.Vmid))
				}
				existingLabel = fmt.Sprintf("Existing VM IDs on %s: %s", node, strings.Join(ids, ", "))
			} else {
				existingLabel = fmt.Sprintf("Existing VM IDs on %s: none", node)
			}
		}
	}

	label := "VM ID"
	if existingLabel != "" {
		label = fmt.Sprintf("%s\n%s", label, existingLabel)
	}

	for {
		value := tui.InputWithExample(label, "123", fmt.Sprintf("%d", defaultValue))
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil && parsed > 0 {
			return parsed
		}
		fmt.Println("Please enter a positive integer.")
	}
}

func uploadVMCloudInitCustomScript(node string, vmid int, storage string, script string) (string, error) {
	node = strings.TrimSpace(node)
	storage = strings.TrimSpace(storage)
	script = strings.TrimSpace(script)
	if node == "" {
		return "", fmt.Errorf("Proxmox node is required for --ci-custom-script")
	}
	if vmid <= 0 {
		return "", fmt.Errorf("VM ID is required for --ci-custom-script")
	}
	if storage == "" {
		storage = "local"
	}
	if script == "" {
		return "", fmt.Errorf("--ci-custom-script requires script content")
	}

	filename := fmt.Sprintf("infractl-vm-%d-user-data.yaml", vmid)
	volid := fmt.Sprintf("%s:snippets/%s", storage, filename)
	cloudConfig := vmCloudInitUserDataFromScript(script)

	sshClient, _, err := initializeRootSSHClient(node, "root")
	if err != nil {
		return "", fmt.Errorf("initialize SSH to upload cloud-init snippet: %w", err)
	}
	defer sshClient.Close()

	encoded := base64.StdEncoding.EncodeToString([]byte(cloudConfig))
	uploadScript := `set -eu
volid=` + shellQuote(volid) + `
path="$(pvesm path "$volid" 2>/dev/null || true)"
if [ -z "$path" ] && [ "${volid%%:*}" = "local" ]; then
  path="/var/lib/vz/${volid#*:}"
fi
if [ -z "$path" ]; then
  echo "Unable to resolve snippet path for $volid. Ensure the storage supports snippets." >&2
  exit 1
fi
mkdir -p "$(dirname "$path")"
printf %s ` + shellQuote(encoded) + ` | base64 -d > "$path"
chmod 0644 "$path"`

	out, err := sshClient.Run("sh -c " + shellQuote(uploadScript))
	if err != nil {
		return "", formatSSHExecError(err, out)
	}
	return volid, nil
}

func vmCloudInitUserDataFromScript(script string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(script))
	return `#cloud-config
write_files:
  - path: /var/lib/infractl/custom-init.sh
    permissions: '0700'
    encoding: b64
    content: ` + encoded + `
runcmd:
  - [ /bin/sh, /var/lib/infractl/custom-init.sh ]
`
}

func printVMCreateConnectionInfo(req *vmCreateRequest, vmid int) {
	fmt.Println("VM creation information:")
	fmt.Printf("  VMID: %d\n", vmid)
	if req != nil {
		if strings.TrimSpace(req.Name) != "" {
			fmt.Printf("  Name: %s\n", req.Name)
		}
		if req.TemplateVMID > 0 {
			fmt.Printf("  Template VMID: %d\n", req.TemplateVMID)
		}
		if strings.TrimSpace(req.CIUser) != "" {
			fmt.Printf("  Cloud-init user: %s\n", req.CIUser)
		}
	}
}

func vmCreateBoolValue(cmd *cobra.Command, vp *viper.Viper, key string) bool {
	flagName := strings.ReplaceAll(key, "_", "-")
	if cmd.Flags().Changed(flagName) || vp.InConfig(key) {
		return vp.GetBool(key)
	}

	defaultKey := "proxmox_vm_defaults_" + key
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

func vmCreateIntValue(cmd *cobra.Command, vp *viper.Viper, key string) int {
	flagName := strings.ReplaceAll(key, "_", "-")
	if cmd.Flags().Changed(flagName) || vp.InConfig(key) {
		return vp.GetInt(key)
	}

	defaultKey := "proxmox_vm_defaults_" + key
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

func vmCreateStringValue(cmd *cobra.Command, vp *viper.Viper, key string) string {
	flagName := strings.ReplaceAll(key, "_", "-")
	if cmd.Flags().Changed(flagName) || vp.InConfig(key) {
		return vp.GetString(key)
	}

	defaultKey := "proxmox_vm_defaults_" + key
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

func init() {
	proxmoxVmSubCmd.AddCommand(proxmoxVmCreateCmd)
	proxmoxVmCreateCmd.Flags().StringVar(&configFilePath, "config-file", "", "Path to YAML config file for PVE auth and VM")

	proxmoxVmCreateCmd.Flags().String("host-url", "", "Proxmox host URL")
	proxmoxVmCreateCmd.Flags().String("api-token", "", "Proxmox API token")
	proxmoxVmCreateCmd.Flags().String("pve-node", "", "Proxmox node name")
	proxmoxVmCreateCmd.Flags().IntVar(&createVmID, "vmid", 9000, "VMID to assign to the new VM")
	proxmoxVmCreateCmd.Flags().Int("template-vmid", 0, "Template VMID to clone")
	proxmoxVmCreateCmd.Flags().String("name", "", "Name for the Proxmox VM")
	proxmoxVmCreateCmd.Flags().Int("sockets", 1, "Number of sockets")
	proxmoxVmCreateCmd.Flags().Int("cores", 1, "CPU cores")
	proxmoxVmCreateCmd.Flags().Int("memory", 1024, "Memory in MB")
	proxmoxVmCreateCmd.Flags().String("description", "New VM", "Description for the Proxmox VM")
	proxmoxVmCreateCmd.Flags().String("storage", "", "Target storage for a full clone")
	proxmoxVmCreateCmd.Flags().Bool("full-clone", true, "Create a full clone from the template")
	proxmoxVmCreateCmd.Flags().Bool("start", true, "Start after create")

	proxmoxVmCreateCmd.Flags().String("ci-user", "", "Cloud-init user")
	proxmoxVmCreateCmd.Flags().String("ci-password", "", "Cloud-init password")
	proxmoxVmCreateCmd.Flags().StringSlice("ssh-public-keys", nil, "Authorized SSH public keys for cloud-init")
	proxmoxVmCreateCmd.Flags().String("ipconfig0", "ip=dhcp", "Cloud-init network config for net0")
	proxmoxVmCreateCmd.Flags().String("nameserver", "", "Cloud-init DNS nameserver")
	proxmoxVmCreateCmd.Flags().String("searchdomain", "", "Cloud-init DNS search domain")
	proxmoxVmCreateCmd.Flags().String("ci-snippets-storage", "local", "Storage ID used for cloud-init snippets")
	proxmoxVmCreateCmd.Flags().String("ci-custom-script", "", "Custom shell script to run through cloud-init user-data")
	proxmoxVmCreateCmd.Flags().String("ci-custum-script", "", "Deprecated spelling of --ci-custom-script")
}

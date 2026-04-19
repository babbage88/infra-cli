package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/babbage88/infra-cli/proxmox"
	"github.com/babbage88/infra-cli/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cloudImageTemplateVMID int

type vmCloudImageTemplateRequest struct {
	Node             string
	VMID             int
	Name             string
	ImageURL         string
	Storage          string
	CloudInitStorage string
	MemoryMB         int
	Sockets          int
	Cores            int
	Description      string
	Net0             string
	SCSIHW           string
	DiskBus          string
	BootOrder        string
	Agent            bool
	SerialConsole    bool
	CleanupImage     bool
}

var proxmoxVmCloudImageTemplateCmd = &cobra.Command{
	Use:     "cloud-image-template",
	Aliases: []string{"template-from-image", "create-template", "cloud-template"},
	Short:   "Create a Proxmox VM template from a cloud image URL",
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
		req := vmCloudImageTemplateRequest{
			Node:             vmCreateStringValue(cmd, localViper, "pve_node"),
			VMID:             vmCreateIntValue(cmd, localViper, "vmid"),
			Name:             vmCreateStringValue(cmd, localViper, "name"),
			ImageURL:         vmCreateStringValue(cmd, localViper, "image_url"),
			Storage:          vmCreateStringValue(cmd, localViper, "storage"),
			CloudInitStorage: vmCreateStringValue(cmd, localViper, "cloudinit_storage"),
			MemoryMB:         vmCreateIntValue(cmd, localViper, "memory"),
			Sockets:          vmCreateIntValue(cmd, localViper, "sockets"),
			Cores:            vmCreateIntValue(cmd, localViper, "cores"),
			Description:      vmCreateStringValue(cmd, localViper, "description"),
			Net0:             vmCreateStringValue(cmd, localViper, "net0"),
			SCSIHW:           vmCreateStringValue(cmd, localViper, "scsihw"),
			DiskBus:          vmCreateStringValue(cmd, localViper, "disk_bus"),
			BootOrder:        vmCreateStringValue(cmd, localViper, "boot_order"),
			Agent:            vmCreateBoolValue(cmd, localViper, "agent"),
			SerialConsole:    vmCreateBoolValue(cmd, localViper, "serial_console"),
			CleanupImage:     vmCreateBoolValue(cmd, localViper, "cleanup_image"),
		}
		if cloudImageTemplateVMID != 0 && req.VMID == 0 {
			req.VMID = cloudImageTemplateVMID
		}

		if err := promptForMissingVMCloudImageTemplate(cmd, localViper, &auth, &req); err != nil {
			return err
		}
		if err := validateVMCloudImageTemplateRequest(&req); err != nil {
			return err
		}

		client, err := proxmox.NewClientTokenString(auth.Host, auth.ApiToken, true)
		if err != nil {
			return fmt.Errorf("create proxmox client: %w", err)
		}

		ctx := context.Background()
		fmt.Println("Creating VM shell through the Proxmox API...")
		if err := client.CreateVM(ctx, req.Node, req.VMID, buildVMCloudImageBaseConfig(&req)); err != nil {
			return fmt.Errorf("create VM %d: %w", req.VMID, err)
		}
		if err := waitForVMUnlocked(ctx, client, req.Node, req.VMID, 3*time.Minute); err != nil {
			return err
		}

		fmt.Println("Downloading and importing cloud image over SSH...")
		importedVolID, err := importCloudImageDiskOverSSH(req.Node, &req)
		if err != nil {
			return err
		}

		fmt.Println("Attaching imported disk and cloud-init device through the Proxmox API...")
		if err := client.UpdateVMConfig(ctx, req.Node, req.VMID, buildVMCloudImageDiskConfig(&req, importedVolID)); err != nil {
			return fmt.Errorf("attach imported disk: %w", err)
		}
		if err := waitForVMUnlocked(ctx, client, req.Node, req.VMID, 3*time.Minute); err != nil {
			return err
		}

		fmt.Println("Converting VM to template...")
		if err := client.TemplateVM(ctx, req.Node, req.VMID); err != nil {
			return fmt.Errorf("convert VM %d to template: %w", req.VMID, err)
		}

		printVMCloudImageTemplateInfo(&req, importedVolID)
		return nil
	},
}

func promptForMissingVMCloudImageTemplate(cmd *cobra.Command, vp *viper.Viper, auth *proxmox.Auth, req *vmCloudImageTemplateRequest) error {
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
		req.VMID = mustPromptVMID(auth, req.Node, req.VMID, 9000)
	} else if req.VMID <= 0 {
		req.VMID = mustPromptVMID(auth, req.Node, req.VMID, 9000)
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = tui.InputWithExample("Template name", "ubuntu-2404-cloudimg", "")
	}
	if strings.TrimSpace(req.ImageURL) == "" {
		req.ImageURL = tui.InputWithExample("Cloud image URL", "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img", "")
	}
	if strings.TrimSpace(req.Storage) == "" {
		req.Storage = tui.InputWithExample("VM disk storage", "local-lvm", "local-lvm")
	}
	if strings.TrimSpace(req.CloudInitStorage) == "" {
		req.CloudInitStorage = req.Storage
	}
	return nil
}

func validateVMCloudImageTemplateRequest(req *vmCloudImageTemplateRequest) error {
	if strings.TrimSpace(req.Node) == "" {
		return fmt.Errorf("Proxmox node is required")
	}
	if req.VMID <= 0 {
		return fmt.Errorf("VM ID is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("template name is required")
	}
	if strings.TrimSpace(req.Storage) == "" {
		return fmt.Errorf("storage is required")
	}
	if _, err := url.ParseRequestURI(req.ImageURL); err != nil {
		return fmt.Errorf("invalid image URL %q: %w", req.ImageURL, err)
	}
	if !strings.HasPrefix(strings.ToLower(req.ImageURL), "https://") {
		return fmt.Errorf("image URL must use https")
	}
	if ext := strings.ToLower(path.Ext(cloudImageFilename(req.ImageURL, req.VMID))); ext != ".img" && ext != ".qcow" && ext != ".qcow2" && ext != ".iso" && ext != ".raw" {
		return fmt.Errorf("unsupported image extension %q; expected .img, .qcow, .qcow2, .iso, or .raw", ext)
	}
	if strings.TrimSpace(req.DiskBus) == "" {
		req.DiskBus = "scsi0"
	}
	if !strings.HasPrefix(req.DiskBus, "scsi") && !strings.HasPrefix(req.DiskBus, "virtio") && !strings.HasPrefix(req.DiskBus, "sata") {
		return fmt.Errorf("unsupported disk bus %q; use scsi0, virtio0, or sata0 style values", req.DiskBus)
	}
	if strings.TrimSpace(req.SCSIHW) == "" {
		req.SCSIHW = "virtio-scsi-pci"
	}
	if strings.TrimSpace(req.BootOrder) == "" {
		req.BootOrder = req.DiskBus
	}
	if strings.TrimSpace(req.CloudInitStorage) == "" {
		req.CloudInitStorage = req.Storage
	}
	return nil
}

func buildVMCloudImageBaseConfig(req *vmCloudImageTemplateRequest) *proxmox.ProxmoxQemuVmConfig {
	cfg := &proxmox.ProxmoxQemuVmConfig{
		Name:        req.Name,
		MemoryMB:    json.Number(strconv.Itoa(req.MemoryMB)),
		Sockets:     json.Number(strconv.Itoa(req.Sockets)),
		Cores:       json.Number(strconv.Itoa(req.Cores)),
		Description: req.Description,
		Raw: map[string]string{
			"scsihw": req.SCSIHW,
			"net0":   req.Net0,
		},
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
	if req.Agent {
		cfg.Raw["agent"] = "enabled=1"
	}
	if req.SerialConsole {
		cfg.Raw["serial0"] = "socket"
		cfg.Raw["vga"] = "serial0"
	}
	return cfg
}

func buildVMCloudImageDiskConfig(req *vmCloudImageTemplateRequest, importedVolID string) *proxmox.ProxmoxQemuVmConfig {
	return &proxmox.ProxmoxQemuVmConfig{Raw: map[string]string{
		req.DiskBus: importedVolID,
		"ide2":      fmt.Sprintf("%s:cloudinit", req.CloudInitStorage),
		"boot":      "order=" + req.BootOrder,
	}}
}

func importCloudImageDiskOverSSH(defaultHost string, req *vmCloudImageTemplateRequest) (string, error) {
	sshClient, _, err := initializeRootSSHClient(defaultHost, "root")
	if err != nil {
		return "", fmt.Errorf("initialize SSH for cloud image import: %w", err)
	}
	defer sshClient.Close()

	out, err := sshClient.Run("sh -c " + shellQuote(renderVMCloudImageImportScript(req)))
	if err != nil {
		return "", formatSSHExecError(err, out)
	}

	volid := parseCloudImageImportVolID(string(out))
	if volid == "" {
		return "", fmt.Errorf("cloud image import completed but no imported volume ID was detected in output: %s", strings.TrimSpace(string(out)))
	}
	return volid, nil
}

func renderVMCloudImageImportScript(req *vmCloudImageTemplateRequest) string {
	cleanup := "true"
	if !req.CleanupImage {
		cleanup = "false"
	}

	return `set -eu
vmid=` + shellQuote(strconv.Itoa(req.VMID)) + `
image_url=` + shellQuote(req.ImageURL) + `
storage=` + shellQuote(req.Storage) + `
filename=` + shellQuote(cloudImageFilename(req.ImageURL, req.VMID)) + `
cleanup_image=` + cleanup + `
workdir="$(mktemp -d /var/tmp/infractl-cloud-image.XXXXXX)"
cleanup() {
  if [ "$cleanup_image" = "true" ]; then
    rm -rf "$workdir"
  else
    echo "IMAGE_PATH=$workdir/$filename"
  fi
}
trap cleanup EXIT
image_path="$workdir/$filename"
if command -v curl >/dev/null 2>&1; then
  curl -fL --retry 3 --connect-timeout 20 -o "$image_path" "$image_url"
elif command -v wget >/dev/null 2>&1; then
  wget -O "$image_path" "$image_url"
else
  echo "curl or wget is required on the Proxmox node to download cloud images" >&2
  exit 1
fi
before="$(qm config "$vmid" | sed -n 's/^\(unused[0-9][0-9]*\):.*/\1/p' | sort | tr '\n' ' ')"
qm importdisk "$vmid" "$image_path" "$storage"
after="$(qm config "$vmid")"
imported="$(printf '%s\n' "$after" | awk -v before="$before" '
  BEGIN { split(before, existing, " "); for (i in existing) seen[existing[i]]=1 }
  /^unused[0-9]+:/ {
    key=$1
    sub(/:$/, "", key)
    if (!seen[key]) {
      sub(/^[^:]+:[[:space:]]*/, "", $0)
      print $0
      exit
    }
  }
')"
if [ -z "$imported" ]; then
  imported="$(printf '%s\n' "$after" | awk '/^unused[0-9]+:/ { sub(/^[^:]+:[[:space:]]*/, "", $0); value=$0 } END { print value }')"
fi
if [ -z "$imported" ]; then
  echo "Unable to determine imported disk volume from qm config output" >&2
  exit 1
fi
echo "IMPORTED_VOLID=$imported"`
}

func cloudImageFilename(rawURL string, vmid int) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err == nil && strings.TrimSpace(parsed.Path) != "" {
		filename := path.Base(parsed.Path)
		if filename != "." && filename != "/" && filename != "" {
			return filename
		}
	}
	return fmt.Sprintf("cloud-image-%d.img", vmid)
}

func parseCloudImageImportVolID(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "IMPORTED_VOLID=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "IMPORTED_VOLID="))
		}
	}
	return ""
}

func printVMCloudImageTemplateInfo(req *vmCloudImageTemplateRequest, importedVolID string) {
	fmt.Println("VM template created:")
	fmt.Printf("  VMID: %d\n", req.VMID)
	fmt.Printf("  Name: %s\n", req.Name)
	fmt.Printf("  Imported disk: %s\n", importedVolID)
	fmt.Printf("  Cloud-init storage: %s\n", req.CloudInitStorage)
}

func init() {
	proxmoxVmSubCmd.AddCommand(proxmoxVmCloudImageTemplateCmd)
	proxmoxVmCloudImageTemplateCmd.Flags().StringVar(&configFilePath, "config-file", "", "Path to YAML config file for PVE auth and VM")
	proxmoxVmCloudImageTemplateCmd.Flags().String("host-url", "", "Proxmox host URL")
	proxmoxVmCloudImageTemplateCmd.Flags().String("api-token", "", "Proxmox API token")
	proxmoxVmCloudImageTemplateCmd.Flags().String("pve-node", "", "Proxmox node name")
	proxmoxVmCloudImageTemplateCmd.Flags().IntVar(&cloudImageTemplateVMID, "vmid", 9000, "VMID to assign to the template")
	proxmoxVmCloudImageTemplateCmd.Flags().String("name", "", "Name for the Proxmox VM template")
	proxmoxVmCloudImageTemplateCmd.Flags().String("image-url", "", "HTTPS URL for a cloud image file such as .img, .qcow2, or .iso")
	proxmoxVmCloudImageTemplateCmd.Flags().String("storage", "", "Target Proxmox storage for the imported VM disk")
	proxmoxVmCloudImageTemplateCmd.Flags().String("cloudinit-storage", "", "Storage ID for the cloud-init disk; defaults to --storage")
	proxmoxVmCloudImageTemplateCmd.Flags().Int("sockets", 1, "Number of sockets")
	proxmoxVmCloudImageTemplateCmd.Flags().Int("cores", 1, "CPU cores")
	proxmoxVmCloudImageTemplateCmd.Flags().Int("memory", 2048, "Memory in MB")
	proxmoxVmCloudImageTemplateCmd.Flags().String("description", "Cloud image VM template", "Description for the Proxmox VM template")
	proxmoxVmCloudImageTemplateCmd.Flags().String("net0", "virtio,bridge=vmbr0", "Primary NIC config")
	proxmoxVmCloudImageTemplateCmd.Flags().String("scsihw", "virtio-scsi-pci", "SCSI controller model")
	proxmoxVmCloudImageTemplateCmd.Flags().String("disk-bus", "scsi0", "Disk bus slot for the imported image")
	proxmoxVmCloudImageTemplateCmd.Flags().String("boot-order", "", "Boot order; defaults to the imported disk bus")
	proxmoxVmCloudImageTemplateCmd.Flags().Bool("agent", true, "Enable the QEMU guest agent flag")
	proxmoxVmCloudImageTemplateCmd.Flags().Bool("serial-console", true, "Configure serial0 socket and vga serial0 for cloud images")
	proxmoxVmCloudImageTemplateCmd.Flags().Bool("cleanup-image", true, "Delete the downloaded image from the Proxmox node after import")
}

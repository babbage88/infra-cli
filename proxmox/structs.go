package proxmox

import "fmt"

// Auth stores the Proxmox API token-based credentials.
type Auth struct {
	Host     string // e.g. "https://proxmox.example.com:8006"
	ApiToken string // Format: "USER@REALM!TOKENID=SECRET"
}

type LxcContainer struct {
	Node          string `json:"node,omitempty"`       // Used in URL, not payload
	VmId          int    `json:"vmid,omitempty"`       // Required
	Hostname      string `json:"hostname,omitempty"`   // Required
	Password      string `json:"password,omitempty"`   // Required if not using SSH key
	OsTemplate    string `json:"ostemplate,omitempty"` // Required (e.g., "local:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst")
	Storage       string `json:"storage,omitempty"`    // Required (storage ID for rootfs)
	RootFsSize    string `json:"rootfs,omitempty"`     // Required (e.g., "8G")
	Memory        int    `json:"memory,omitempty"`     // RAM in MB
	Swap          int    `json:"swap,omitempty"`       // Swap in MB
	Cores         int    `json:"cores,omitempty"`      // CPU cores
	CpuLimit      int    `json:"cpulimit,omitempty"`   // Limit in % of total
	CpuUnits      int    `json:"cpuunits,omitempty"`   // Relative CPU weight
	Net0          string `json:"net0,omitempty"`       // Network config string
	Bridge        string `json:"bridge,omitempty"`     // Optional, if setting bridge separately
	Nameserver    string `json:"nameserver,omitempty"` // DNS
	Searchdomain  string `json:"searchdomain,omitempty"`
	Pool          string `json:"pool,omitempty"` // Optional pool
	Description   string `json:"description,omitempty"`
	Unprivileged  string `json:"unprivileged,omitempty"` // 1 or 0
	Start         string `json:"start,omitempty"`        // 1 to auto-start
	BwLimit       int    `json:"bwlimit,omitempty"`
	Arch          string `json:"arch,omitempty"`            // e.g., "amd64"
	Cmode         string `json:"cmode,omitempty"`           // e.g., "tty"
	Console       string `json:"console,omitempty"`         // 0 or 1
	Debug         int    `json:"debug,omitempty"`           // 0 or 1
	Features      string `json:"features,omitempty"`        // Comma-separated list
	Startup       string `json:"startup,omitempty"`         // Startup order string
	Tags          string `json:"tags,omitempty"`            // Comma-separated tags
	SshPublicKeys string `json:"ssh_public_keys,omitempty"` // SSH keys string
}

func (lxc *LxcContainer) ToFormParams() map[string]string {
	params := make(map[string]string)

	if lxc.VmId != 0 {
		params["vmid"] = fmt.Sprintf("%d", lxc.VmId)
	}
	if lxc.Hostname != "" {
		params["hostname"] = lxc.Hostname
	}
	if lxc.Password != "" {
		params["password"] = lxc.Password
	}
	if lxc.OsTemplate != "" {
		params["ostemplate"] = lxc.OsTemplate
	}
	if lxc.SshPublicKeys != "" {
		params["ssh-public-keys"] = lxc.SshPublicKeys
	}
	if lxc.Storage != "" {
		params["storage"] = lxc.Storage
	}
	if lxc.RootFsSize != "" && lxc.Storage != "" {
		params["rootfs"] = fmt.Sprintf("%s:%s", lxc.Storage, lxc.RootFsSize)
	}
	if lxc.Memory != 0 {
		params["memory"] = fmt.Sprintf("%d", lxc.Memory)
	}
	if lxc.Swap != 0 {
		params["swap"] = fmt.Sprintf("%d", lxc.Swap)
	}
	if lxc.Cores != 0 {
		params["cores"] = fmt.Sprintf("%d", lxc.Cores)
	}
	if lxc.CpuLimit != 0 {
		params["cpulimit"] = fmt.Sprintf("%d", lxc.CpuLimit)
	}
	if lxc.CpuUnits != 0 {
		params["cpuunits"] = fmt.Sprintf("%d", lxc.CpuUnits)
	}
	if lxc.Net0 != "" {
		params["net0"] = lxc.Net0
	}
	if lxc.Arch != "" {
		params["arch"] = lxc.Arch
	}
	if lxc.Cmode != "" {
		params["cmode"] = lxc.Cmode
	}
	if lxc.Start != "" {
		params["start"] = lxc.Start
	}
	if lxc.Console != "" {
		params["console"] = lxc.Console
	}
	if lxc.Unprivileged != "" {
		params["unprivileged"] = lxc.Unprivileged
	}

	return params
}

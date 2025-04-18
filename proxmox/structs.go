package proxmox

// Auth stores the Proxmox API token-based credentials.
type Auth struct {
	Host     string // e.g. "https://proxmox.example.com:8006"
	ApiToken string // Format: "USER@REALM!TOKENID=SECRET"
}

type LxcContainer struct {
	Node          string `json:"node"`       // Used in URL, not payload
	VmId          int    `json:"vmid"`       // Required
	Hostname      string `json:"hostname"`   // Required
	Password      string `json:"password"`   // Required if not using SSH key
	Ostemplate    string `json:"ostemplate"` // Required (e.g., "local:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst")
	Storage       string `json:"storage"`    // Required (storage ID for rootfs)
	RootFsSize    string `json:"rootfs"`     // Required (e.g., "8G")
	Memory        int    `json:"memory"`     // RAM in MB
	Swap          int    `json:"swap"`       // Swap in MB
	Cores         int    `json:"cores"`      // CPU cores
	CpuLimit      int    `json:"cpulimit"`   // Limit in % of total
	CpuUnits      int    `json:"cpuunits"`   // Relative CPU weight
	Net0          string `json:"net0"`       // Network config string
	Bridge        string `json:"bridge"`     // Optional, if setting bridge separately
	Nameserver    string `json:"nameserver"` // DNS
	Searchdomain  string `json:"searchdomain"`
	Pool          string `json:"pool"` // Optional pool
	Description   string `json:"description"`
	Unprivileged  int    `json:"unprivileged"` // 1 or 0
	Start         int    `json:"start"`        // 1 to auto-start
	Autostart     int    `json:"autostart"`    // 1 to enable autostart on boot
	BwLimit       int    `json:"bwlimit"`
	Arch          string `json:"arch"`            // e.g., "amd64"
	Cmode         string `json:"cmode"`           // e.g., "tty"
	Console       int    `json:"console"`         // 0 or 1
	Debug         int    `json:"debug"`           // 0 or 1
	Features      string `json:"features"`        // Comma-separated list
	Startup       string `json:"startup"`         // Startup order string
	Tags          string `json:"tags"`            // Comma-separated tags
	SshPublicKeys string `json:"ssh-public-keys"` // SSH keys string
}

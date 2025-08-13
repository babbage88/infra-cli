package proxmox

type ProxmoxStorageType string
type ProxmoxStorageContentType string
type ProxmoxStorageEnabledContent map[ProxmoxStorageContentType]bool

type ProxmoxResource interface {
	ProxmoxQemuVmConfig | LxcContainer
}

const (
	Directory           ProxmoxStorageType = "directory"
	LVM                 ProxmoxStorageType = "lvm"
	LvmThin             ProxmoxStorageType = "lvm-thin"
	BTRFS               ProxmoxStorageType = "btrfs"
	NFS                 ProxmoxStorageType = "nfs"
	SmbCifs             ProxmoxStorageType = "cifs"
	GlusterFs           ProxmoxStorageType = "glusterfs"
	CephFs              ProxmoxStorageType = "cephfs"
	RBD                 ProxmoxStorageType = "rbd"
	ZfsOverIscsi        ProxmoxStorageType = "zfs-iscsi"
	ZFS                 ProxmoxStorageType = "zfs"
	ProxmoxBackupServer ProxmoxStorageType = "pbs"
)

const (
	Backup            ProxmoxStorageContentType = "backup"
	Iso               ProxmoxStorageContentType = "iso"
	VmDiskImages      ProxmoxStorageContentType = "image"
	CloudInitSnippets ProxmoxStorageContentType = "snippets"
	LxcTemplates      ProxmoxStorageContentType = "vztmpl"
	ContainerRootDir  ProxmoxStorageContentType = "rootdir"
)

type ProxmoxStoragePool struct {
	Name         string                       `json:"name"`
	Type         ProxmoxStorageType           `json:"type"`
	Capabilities ProxmoxStorageEnabledContent `json:"capabilities"`
	Path         string                       `json:"path"`
	Server       string                       `json:"server"`
	Shared       bool                         `json:"shared"`
	TotalBytes   int                          `json:"totalBytes"`
	Used         int                          `json:"used"`
	Enabled      bool                         `json:"enabled"`
}

package proxmox

import coreproxmox "github.com/babbage88/infra-core/proxmox"

type APIError = coreproxmox.APIError
type AuthMethod = coreproxmox.AuthMethod
type Client = coreproxmox.Client
type TLSConfig = coreproxmox.TLSConfig

type ProxmoxVmId = coreproxmox.ProxmoxVmId
type ProxmoxVmName = coreproxmox.ProxmoxVmName
type ProxmoxNode = coreproxmox.ProxmoxNode

type QemuVm = coreproxmox.QemuVm
type ProxmoxQemuVmConfig = coreproxmox.ProxmoxQemuVmConfig
type Auth = coreproxmox.Auth
type LxcContainer = coreproxmox.LxcContainer

type LxcSSHForceOptions = coreproxmox.LxcSSHForceOptions
type LxcLogSink = coreproxmox.LxcLogSink
type LxcLogKind = coreproxmox.LxcLogKind
type LxcLogEntry = coreproxmox.LxcLogEntry

type ProxmoxStorageType = coreproxmox.ProxmoxStorageType
type ProxmoxStorageContentType = coreproxmox.ProxmoxStorageContentType
type ProxmoxStorageEnabledContent = coreproxmox.ProxmoxStorageEnabledContent
type ProxmoxResource = coreproxmox.ProxmoxResource
type ProxmoxStoragePool = coreproxmox.ProxmoxStoragePool
type NodeStorage = coreproxmox.NodeStorage
type StorageContentItem = coreproxmox.StorageContentItem

type QemuCloneRequest = coreproxmox.QemuCloneRequest

const (
	AuthPassword = coreproxmox.AuthPassword
	AuthToken    = coreproxmox.AuthToken
)

const (
	LxcLogStatus  = coreproxmox.LxcLogStatus
	LxcLogCommand = coreproxmox.LxcLogCommand
)

const (
	Directory           = coreproxmox.Directory
	LVM                 = coreproxmox.LVM
	LvmThin             = coreproxmox.LvmThin
	BTRFS               = coreproxmox.BTRFS
	NFS                 = coreproxmox.NFS
	SmbCifs             = coreproxmox.SmbCifs
	GlusterFs           = coreproxmox.GlusterFs
	CephFs              = coreproxmox.CephFs
	RBD                 = coreproxmox.RBD
	ZfsOverIscsi        = coreproxmox.ZfsOverIscsi
	ZFS                 = coreproxmox.ZFS
	ProxmoxBackupServer = coreproxmox.ProxmoxBackupServer
)

const (
	Backup            = coreproxmox.Backup
	Iso               = coreproxmox.Iso
	VmDiskImages      = coreproxmox.VmDiskImages
	CloudInitSnippets = coreproxmox.CloudInitSnippets
	LxcTemplates      = coreproxmox.LxcTemplates
	ContainerRootDir  = coreproxmox.ContainerRootDir
)

var (
	ParseQemuVmConfig                        = coreproxmox.ParseQemuVmConfig
	ForceLxcSSHReadiness                     = coreproxmox.ForceLxcSSHReadiness
	ForceLxcSSHReadinessWithLog              = coreproxmox.ForceLxcSSHReadinessWithLog
	DetectLxcOSTypeOverSSHWithLog            = coreproxmox.DetectLxcOSTypeOverSSHWithLog
	EnsureLxcNetworkingStartedOverSSHWithLog = coreproxmox.EnsureLxcNetworkingStartedOverSSHWithLog
	EnsureLxcSSHServerAndUsersOverSSH        = coreproxmox.EnsureLxcSSHServerAndUsersOverSSH
	EnsureLxcSSHServerAndUsersOverSSHWithLog = coreproxmox.EnsureLxcSSHServerAndUsersOverSSHWithLog
	RunPctExecShellScript                    = coreproxmox.RunPctExecShellScript
	RunPctExecShellScriptWithLog             = coreproxmox.RunPctExecShellScriptWithLog
	RunRemoteQuotedCommandWithLog            = coreproxmox.RunRemoteQuotedCommandWithLog
	WaitForLxcRunningOverSSH                 = coreproxmox.WaitForLxcRunningOverSSH
	WaitForLxcRunningOverSSHWithLog          = coreproxmox.WaitForLxcRunningOverSSHWithLog
	WaitForLxcIPv4OverSSH                    = coreproxmox.WaitForLxcIPv4OverSSH
	WaitForLxcIPv4OverSSHWithLog             = coreproxmox.WaitForLxcIPv4OverSSHWithLog
	GetLxcPrimaryIPv4OverSSH                 = coreproxmox.GetLxcPrimaryIPv4OverSSH
	GetLxcPrimaryIPv4OverSSHWithLog          = coreproxmox.GetLxcPrimaryIPv4OverSSHWithLog
	VerifyLxcSSHFromProxmoxNode              = coreproxmox.VerifyLxcSSHFromProxmoxNode
	CreateLXCContainer                       = coreproxmox.CreateLXCContainer
	NewClient                                = coreproxmox.NewClient
	NewClientPassword                        = coreproxmox.NewClientPassword
	NewClientToken                           = coreproxmox.NewClientToken
	NewClientTokenString                     = coreproxmox.NewClientTokenString
	ParseAPIToken                            = coreproxmox.ParseAPIToken
)

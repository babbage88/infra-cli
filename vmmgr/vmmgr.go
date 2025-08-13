package vmmgr

type VmIdentifier interface {
	~int | ~string
}

type VmManager[T VmIdentifier] interface {
	StartVm(ids []T) error
	StopVm(ids []T) error
	ResetVm(ids []T) error
	UpdateVmConfiguration(ids []T) error
}

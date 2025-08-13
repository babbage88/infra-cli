package cmd

import (
	"encoding/json"
	"strconv"

	"github.com/babbage88/infra-cli/proxmox"
)

type ProxmoxVmCommandFlags struct {
	ApiUrl              string            `json:"proxmox_api_url,omitempty"`
	Name                string            `json:"name,omitempty"`
	MemoryMB            int               `json:"memory,omitempty"`
	Sockets             int               `json:"sockets,omitempty"`
	Cores               int               `json:"cores,omitempty"`
	Description         string            `json:"description,omitempty"`
	Raw                 map[string]string `json:"extraConfig,omitempty"`
	AuthTokenOrUsername string            `json:"authTokenOrUsername,omitempty"`
	PasswordOrSecret    string            `json:"passwordOrSecret,omitempty"`
	PveNode             string            `json:"pveNode,omitempty"`
	PvePort             string            `json:"pvePort,omitempty"`
	SkipTls             bool              `json:"skipTls,omitempty"`
	UseToken            bool              `json:"useToken,omitempty"`
}

func intToJsonNumber(i int) json.Number {
	strInt := strconv.Itoa(i)
	return json.Number(strInt)
}

func (v *ProxmoxVmCommandFlags) ParseVMConfigTyped() *proxmox.ProxmoxQemuVmConfig {
	vmConfig := proxmox.ProxmoxQemuVmConfig{
		Name:        v.Name,
		MemoryMB:    intToJsonNumber(v.MemoryMB),
		Sockets:     intToJsonNumber(v.Sockets),
		Cores:       intToJsonNumber(v.Cores),
		Description: v.Description,
		Raw:         v.Raw,
	}

	return &vmConfig
}

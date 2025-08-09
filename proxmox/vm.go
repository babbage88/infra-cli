package proxmox

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// VMConfigTyped represents common VM configuration fields.
type VMConfigTyped struct {
	Name        string `json:"name,omitempty"`
	MemoryMB    int    `json:"memory,omitempty"`
	Sockets     int    `json:"sockets,omitempty"`
	Cores       int    `json:"cores,omitempty"`
	Description string `json:"description,omitempty"`
	// Raw holds additional fields not mapped above.
	Raw map[string]string
}

// ToParams converts VMConfigTyped to API form parameters.
func (cfg *VMConfigTyped) ToParams() url.Values {
	params := url.Values{}

	if cfg.Name != "" {
		params.Set("name", cfg.Name)
	}
	if cfg.MemoryMB > 0 {
		params.Set("memory", strconv.Itoa(cfg.MemoryMB))
	}
	if cfg.Sockets > 0 {
		params.Set("sockets", strconv.Itoa(cfg.Sockets))
	}
	if cfg.Cores > 0 {
		params.Set("cores", strconv.Itoa(cfg.Cores))
	}
	if cfg.Description != "" {
		params.Set("description", cfg.Description)
	}

	for k, v := range cfg.Raw {
		if v != "" {
			params.Set(k, v)
		}
	}
	return params
}

// GetVMConfig returns a typed VM config.
func (c *Client) GetVMConfig(ctx context.Context, node string, vmid int) (*VMConfigTyped, error) {
	path := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)
	var raw map[string]interface{}
	if err := c.do(ctx, "GET", path, nil, nil, false, &raw); err != nil {
		return nil, err
	}

	cfg := &VMConfigTyped{Raw: make(map[string]string)}
	for k, v := range raw {
		switch k {
		case "name":
			cfg.Name = fmt.Sprintf("%v", v)
		case "memory":
			cfg.MemoryMB = int(v.(float64))
		case "sockets":
			cfg.Sockets = int(v.(float64))
		case "cores":
			cfg.Cores = int(v.(float64))
		case "description":
			cfg.Description = fmt.Sprintf("%v", v)
		default:
			cfg.Raw[k] = fmt.Sprintf("%v", v)
		}
	}
	return cfg, nil
}

// UpdateVMConfig updates a VM configuration using VMConfigTyped.
func (c *Client) UpdateVMConfig(ctx context.Context, node string, vmid int, cfg *VMConfigTyped) error {
	if cfg == nil {
		return fmt.Errorf("VMConfigTyped cannot be nil")
	}
	params := cfg.ToParams()
	path := fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)
	headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	return c.do(ctx, "PUT", path, strings.NewReader(params.Encode()), headers, true, nil)
}

// SetMemory updates VM memory (MB) using VMConfigTyped.
func (c *Client) SetMemory(ctx context.Context, node string, vmid int, memMB int) error {
	cfg := &VMConfigTyped{MemoryMB: memMB}
	return c.UpdateVMConfig(ctx, node, vmid, cfg)
}

// SetCores updates CPU cores using VMConfigTyped.
func (c *Client) SetCores(ctx context.Context, node string, vmid int, cores int) error {
	cfg := &VMConfigTyped{Cores: cores}
	return c.UpdateVMConfig(ctx, node, vmid, cfg)
}

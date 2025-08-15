package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/babbage88/infra-cli/internal/pretty"
)

// CreateVM creates a new VM on a given Proxmox node using VMConfigTyped.
// vmid must be a unique unused VM ID.
func (c *Client) CreateVM(ctx context.Context, node string, vmid int, cfg *ProxmoxQemuVmConfig) error {
	if cfg == nil {
		return fmt.Errorf("VMConfigTyped cannot be nil")
	}
	if vmid <= 0 {
		return fmt.Errorf("invalid VMID: %d", vmid)
	}

	params := cfg.ToParams()
	params.Set("vmid", fmt.Sprintf("%d", vmid))

	path := fmt.Sprintf("%s/%s/qemu", apiNodesPath, url.PathEscape(node))
	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	// The Proxmox API expects POST for creating a VM.
	return c.do(ctx, "POST", path, strings.NewReader(params.Encode()), headers, true, nil)
}

// ToParams converts VMConfigTyped to API form parameters.
func (cfg *ProxmoxQemuVmConfig) ToParams() url.Values {
	params := url.Values{}

	if cfg.Name != "" {
		params.Set("name", cfg.Name)
	}
	if cfg.MemoryMB != "" {
		params.Set("memory", cfg.MemoryMB.String())
	}
	if cfg.Sockets != "" {
		params.Set("sockets", cfg.Sockets.String())
	}
	if cfg.Cores != "" {
		params.Set("cores", cfg.Cores.String())
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
func (c *Client) GetVMConfig(ctx context.Context, node string, vmid int) (*ProxmoxQemuVmConfig, error) {
	path := fmt.Sprintf("%s/%s/qemu/%d/config", apiNodesPath, url.PathEscape(node), vmid)

	// Use a json.Number-aware decoder to preserve number formatting
	var raw map[string]any
	if err := c.do(ctx, "GET", path, nil, nil, false, &raw); err != nil {
		return nil, err
	}

	cfg := &ProxmoxQemuVmConfig{Raw: make(map[string]string)}
	for k, v := range raw {
		switch k {
		case "name":
			cfg.Name = fmt.Sprintf("%v", v)
		case "memory":
			cfg.MemoryMB = toJSONNumber(v)
		case "sockets":
			cfg.Sockets = toJSONNumber(v)
		case "cores":
			cfg.Cores = toJSONNumber(v)
		case "description":
			cfg.Description = fmt.Sprintf("%v", v)
		default:
			cfg.Raw[k] = fmt.Sprintf("%v", v)
		}
	}
	return cfg, nil
}

func (c *Client) StartVM(ctx context.Context, node string, vmid int) (map[string]any, error) {
	path := fmt.Sprintf("%s/%s/qemu/%d%s", apiNodesPath, url.PathEscape(node), vmid, apiVmStartSubPath)

	var resp map[string]any

	slog.Info("Sending http client POST to start vm", slog.String("node", node), slog.Int("vmid", vmid), slog.String("path", path))
	if err := c.do(ctx, http.MethodPost, path, nil, nil, false, &resp); err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Client) StopVM(ctx context.Context, node string, vmid int) (map[string]any, error) {
	path := fmt.Sprintf("%s/%s/qemu/%d%s", apiNodesPath, url.PathEscape(node), vmid, apiVmStopSubPath)

	var resp map[string]any

	slog.Info("Sending http client POST to stop vm", slog.String("node", node), slog.Int("vmid", vmid), slog.String("path", path))
	if err := c.do(ctx, http.MethodPost, path, nil, nil, false, &resp); err != nil {
		return nil, err
	}

	return resp, nil
}

func (cfg *ProxmoxQemuVmConfig) PrintJSON() error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal VMConfigTyped: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func (cfg *ProxmoxQemuVmConfig) PrettyPrintJSON() error {
	// Marshal to generic interface
	var data interface{}
	b, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal VMConfigTyped: %w", err)
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return fmt.Errorf("failed to unmarshal for printing: %w", err)
	}

	pretty.PrintColoredJSON(data, 0)
	fmt.Println()
	return nil
}

// Helper to safely convert numeric API values to json.Number
func toJSONNumber(v any) json.Number {
	switch t := v.(type) {
	case json.Number:
		return t
	case int:
		intStr := strconv.Itoa(t)
		return json.Number(intStr)
	case float64:
		return json.Number(fmt.Sprintf("%.0f", t))
	case string:
		return json.Number(t)
	default:
		return ""
	}
}

// UpdateVMConfig updates a VM configuration using VMConfigTyped.
func (c *Client) UpdateVMConfig(ctx context.Context, node string, vmid int, cfg *ProxmoxQemuVmConfig) error {
	if cfg == nil {
		return fmt.Errorf("VMConfigTyped cannot be nil")
	}
	params := cfg.ToParams()
	path := fmt.Sprintf("%s/%s/qemu/%d/config", apiNodesPath, url.PathEscape(node), vmid)
	headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	return c.do(ctx, "PUT", path, strings.NewReader(params.Encode()), headers, true, nil)
}

// SetMemory updates VM memory (MB) using VMConfigTyped.
func (c *Client) SetMemory(ctx context.Context, node string, vmid int, memMB int) error {
	cfg := &ProxmoxQemuVmConfig{MemoryMB: json.Number(fmt.Sprintf("%d", memMB))}
	return c.UpdateVMConfig(ctx, node, vmid, cfg)
}

// SetCores updates CPU cores using VMConfigTyped.
func (c *Client) SetCores(ctx context.Context, node string, vmid int, cores int) error {
	cfg := &ProxmoxQemuVmConfig{Cores: json.Number(fmt.Sprintf("%d", cores))}
	return c.UpdateVMConfig(ctx, node, vmid, cfg)
}

func (c *Client) ListVMs(ctx context.Context, node string, full bool) ([]QemuVm, error) {
	var fullInt int = 0
	if full {
		fullInt = int(1)
	}
	url := fmt.Sprintf("%s/api2/json/nodes/%s/qemu?full=%d", c.baseURL, node, fullInt)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Attach auth
	if c.authMethod == AuthToken {
		req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.username, c.password))
	} else {
		// If using ticket/session-based auth
		req.Header.Set("Cookie", fmt.Sprintf("PVEAuthCookie=%s", c.authTicket))
		req.Header.Set("CSRFPreventionToken", c.csrfToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []QemuVm `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding data: %w", err)
	}

	return result.Data, nil
}

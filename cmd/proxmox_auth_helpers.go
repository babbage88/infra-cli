package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/babbage88/infra-cli/proxmox"
	"github.com/babbage88/infra-cli/tui"
	"github.com/goccy/go-yaml"
	"github.com/spf13/viper"
)

type proxmoxTokenParts struct {
	TokenID string
	Secret  string
}

func normalizeProxmoxConfigValues(vp *viper.Viper) {
	if vp == nil {
		return
	}

	tokenID, secret := resolveConfiguredProxmoxTokenParts(vp)
	if tokenID != "" {
		vp.Set("proxmox_api_token", tokenID)
	}
	if secret != "" {
		vp.Set("proxmox_api_secret", secret)
	}
	if vp.GetString("api_token") == "" && tokenID != "" && secret != "" {
		vp.Set("api_token", fmt.Sprintf("%s=%s", tokenID, secret))
	}
	if vp.GetString("proxmox_api_url") == "" && vp.GetString("proxmox_host") != "" {
		vp.Set("proxmox_api_url", strings.TrimSpace(vp.GetString("proxmox_host")))
	}
	if vp.GetString("host_url") == "" && vp.GetString("proxmox_api_url") != "" {
		vp.Set("host_url", strings.TrimSpace(vp.GetString("proxmox_api_url")))
	}
}

func resolveConfiguredProxmoxTokenParts(vp *viper.Viper) (string, string) {
	if vp == nil {
		return "", ""
	}

	tokenID := decodeMaybeBase64String(vp.GetString("proxmox_api_token"))
	secret := decodeMaybeBase64String(vp.GetString("proxmox_api_secret"))

	if tokenID != "" && secret == "" {
		if parsedTokenID, parsedSecret, err := proxmox.ParseAPIToken(tokenID); err == nil {
			tokenID = parsedTokenID
			secret = parsedSecret
		}
	}

	if tokenID == "" && secret == "" {
		apiToken := decodeMaybeBase64String(vp.GetString("api_token"))
		if apiToken != "" {
			if parsedTokenID, parsedSecret, err := proxmox.ParseAPIToken(apiToken); err == nil {
				tokenID = parsedTokenID
				secret = parsedSecret
			}
		}
	}

	return strings.TrimSpace(tokenID), strings.TrimSpace(secret)
}

func decodeMaybeBase64String(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		decoded, err := enc.DecodeString(value)
		if err != nil || len(decoded) == 0 {
			continue
		}
		if isReadableText(decoded) {
			return string(decoded)
		}
	}

	return value
}

func isReadableText(data []byte) bool {
	for _, b := range data {
		if b == '\n' || b == '\r' || b == '\t' {
			continue
		}
		if b < 32 || b > 126 {
			return false
		}
	}
	return true
}

func rootConfiguredProxmoxTokenParts() (proxmoxTokenParts, string, bool) {
	if rootViperCfg == nil {
		return proxmoxTokenParts{}, "", false
	}

	tokenID, secret := resolveConfiguredProxmoxTokenParts(rootViperCfg)
	if tokenID == "" || secret == "" {
		return proxmoxTokenParts{}, "", false
	}

	return proxmoxTokenParts{TokenID: tokenID, Secret: secret}, rootViperCfg.ConfigFileUsed(), true
}

func promptForProxmoxTokenParts(defaults proxmoxTokenParts) proxmoxTokenParts {
	tokenID := tui.InputWithExample("Proxmox API token ID", "root@pam!infractl-cli", defaults.TokenID)
	secret := tui.PasswordWithExample("Proxmox API token secret", "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", defaults.Secret)
	return proxmoxTokenParts{
		TokenID: strings.TrimSpace(tokenID),
		Secret:  strings.TrimSpace(secret),
	}
}

func combinedProxmoxToken(parts proxmoxTokenParts) string {
	if strings.TrimSpace(parts.TokenID) == "" || strings.TrimSpace(parts.Secret) == "" {
		return ""
	}
	return fmt.Sprintf("%s=%s", strings.TrimSpace(parts.TokenID), strings.TrimSpace(parts.Secret))
}

func writeRootConfigValues(updates map[string]string) error {
	_, err := writeConfigValues(filepath.Join(GetConfigPath(), "default.yaml"), updates)
	return err
}

func writeDefaultRootConfigValues(updates map[string]string) (string, error) {
	return writeConfigValues(filepath.Join(GetConfigPath(), "default.yaml"), updates)
}

func writeConfigValues(targetPath string, updates map[string]string) (string, error) {
	if strings.TrimSpace(targetPath) == "" {
		targetPath = filepath.Join(GetConfigPath(), "default.yaml")
	}

	existing := make(map[string]any)
	if data, err := os.ReadFile(targetPath); err == nil {
		switch strings.ToLower(filepath.Ext(targetPath)) {
		case ".json":
			if err := json.Unmarshal(data, &existing); err != nil {
				return "", fmt.Errorf("decode json config %q: %w", targetPath, err)
			}
		case ".toml":
			if err := toml.Unmarshal(data, &existing); err != nil {
				return "", fmt.Errorf("decode toml config %q: %w", targetPath, err)
			}
		default:
			if err := yaml.Unmarshal(data, &existing); err != nil {
				return "", fmt.Errorf("decode yaml config %q: %w", targetPath, err)
			}
		}
	}

	for key, value := range updates {
		existing[key] = value
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}

	file, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("open config file %q: %w", targetPath, err)
	}
	defer file.Close()

	switch strings.ToLower(filepath.Ext(targetPath)) {
	case ".json":
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(existing); err != nil {
			return "", fmt.Errorf("write json config %q: %w", targetPath, err)
		}
	case ".toml":
		if err := toml.NewEncoder(file).Encode(existing); err != nil {
			return "", fmt.Errorf("write toml config %q: %w", targetPath, err)
		}
	default:
		encoder := yaml.NewEncoder(file)
		defer encoder.Close()
		if err := encoder.Encode(existing); err != nil {
			return "", fmt.Errorf("write yaml config %q: %w", targetPath, err)
		}
	}

	if rootViperCfg != nil {
		for key, value := range updates {
			rootViperCfg.Set(key, value)
		}
		normalizeProxmoxConfigValues(rootViperCfg)
	}

	return targetPath, nil
}

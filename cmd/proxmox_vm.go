package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/babbage88/infra-cli/proxmox"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	proxPveNodeFlagVar           string
	proxVmIdFlagVar              []int
	proxPortFlagVar              int
	proxVmSocketFlagVar          int
	proxVmCoresFlagVar           int
	proxMemowryFlagVar           int
	proxoxUserFlagVar            string
	proxmoxPasswordFlagVar       string
	proxVmNameFlagVar            string
	proxVmDescFlagVar            string
	proxmoxApiUrl                string
	proxmoxAuthToken             string
	proxmoxAuthTokenSecret       string
	proxmoxApiAuthBoolVar        bool
	proxmoxIgnoreTLSErrorBoolVar bool
	rootCAPathFlagVar            string
)

// buildVMConfigFromCmd builds a VMConfigTyped containing only flags that were explicitly set.
func buildVMConfigFromCmd(cmd *cobra.Command) (*proxmox.VMConfigTyped, error) {
	cfg := &proxmox.VMConfigTyped{Raw: make(map[string]string)}

	// string flags
	if cmd.Flags().Changed("name") {
		name, _ := cmd.Flags().GetString("name")
		cfg.Name = name
	}
	if cmd.Flags().Changed("description") {
		desc, _ := cmd.Flags().GetString("description")
		cfg.Description = desc
	}

	// int flags -> set json.Number only if changed
	if cmd.Flags().Changed("memory") {
		mem, _ := cmd.Flags().GetInt("memory")
		cfg.MemoryMB = json.Number(strconv.Itoa(mem))
	}
	if cmd.Flags().Changed("sockets") {
		sockets, _ := cmd.Flags().GetInt("sockets")
		cfg.Sockets = json.Number(strconv.Itoa(sockets))
	}
	if cmd.Flags().Changed("cores") {
		cores, _ := cmd.Flags().GetInt("cores")
		cfg.Cores = json.Number(strconv.Itoa(cores))
	}

	// If you have extraRaw flags, check them here and set cfg.Raw[...] as needed.
	// e.g. if cmd.Flags().Changed("net0") { v, _ := cmd.Flags().GetString("net0"); cfg.Raw["net0"] = v }

	// If Raw map is empty, set to nil to match your desired PrintJSON output
	if len(cfg.Raw) == 0 {
		cfg.Raw = nil
	}

	return cfg, nil
}

func bindLocalFlags(cmd *cobra.Command, vp *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		key := strings.ReplaceAll(f.Name, "-", "_")
		if cmd.Flags().Changed(f.Name) {
			slog.Debug("cobra cmd flag has been changed, binding to <F2>viper.Viper", "Name", f.Name)
			_ = vp.BindPFlag(key, f)
		}
	})
	slog.Debug("Debug vp config", slog.String("proxmox_api_token", vp.GetString("proxmox_api_token")))
}

func loadProxmoxConfigFile(path string, vp *viper.Viper) error {
	if path != "" {
		vp.SetConfigFile(path)
		if err := vp.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		slog.Debug("Debug vp config", slog.String("proxmox_api_token", vp.GetString("proxmox_api_token")))
		apiCheck := vp.GetString("api_token")
		err := validateProxmoxApiToken(apiCheck, vp)
		if err != nil {
			slog.Error("error validating api token", "error", err.Error())
			return err
		}
		return nil
	}

	return nil
}

func validateProxmoxApiToken(apiCheck string, vp *viper.Viper) error {
	if apiCheck == "" {
		apiTokenFromRoot := rootViperCfg.GetString("proxmox_api_token")
		switch len(apiTokenFromRoot) {
		case 0:
			return fmt.Errorf("no proxmox api token supplied")
		default:
			vp.Set("api_token", apiTokenFromRoot)
			return nil
		}
	} else {
		return nil
	}
}

var proxmoxVmSubCmd = &cobra.Command{
	Use:   "vm",
	Short: "Commands for creation and management of Proxmox (QEMU) VMs",
}

func init() {
	proxmoxSubCmd.AddCommand(proxmoxVmSubCmd)
}

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

func newProxmoxClientFromViperConfig(vp *viper.Viper) (*proxmox.Client, error) {
	var useToken bool
	var skipTLS bool
	var pveUserOrToken string
	var pveSecretOrPassword string

	proxmoxApiUrl = vp.GetString("proxmox_api_url")
	useToken = vp.GetBool("use_token")
	skipTLS = vp.GetBool("skip_tls")
	pveUserOrToken = vp.GetString("proxmox_api_token")
	pveSecretOrPassword = vp.GetString("proxmox_api_secret")
	noTokenSupplied := pveUserOrToken == "" && pveSecretOrPassword == ""
	passwordSupplied := vp.GetString("password") != ""

	// If no API Auth token is supplied and the viper use_token value is false,
	// username and password will be tried. All this validation is because viper is
	// not great at detecting the a cmd.Flags.BoolVar() has been set by the user. So
	// the config-file bools were being incorrectly overidden by the cmd.Flag default.

	if noTokenSupplied && !useToken {
		if passwordSupplied {
			pveUserOrToken = vp.GetString("username")
			pveSecretOrPassword = vp.GetString("password")
		} else {
			return nil, fmt.Errorf("No API token or password has been supplied..")
		}
	}

	// if no proxmox_api_url has been set, defult to the pve_node and the pve_port (defaults: 8006)
	if proxmoxApiUrl == "" {
		proxmoxApiUrl = fmt.Sprintf("https://%s:%d", vp.GetString("pve_node"), vp.GetInt("pve_port"))
	}

	slog.Info("Proxmox API URL", "url", proxmoxApiUrl)
	slog.Info("Proxmox Auth Token ID", "token", pveUserOrToken)

	return proxmox.NewClient(proxmoxApiUrl, pveUserOrToken, pveSecretOrPassword, skipTLS, useToken)

}

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
			slog.Debug("cobra cmd flag has been changed, binding to viper.Viper", "Name", f.Name)
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

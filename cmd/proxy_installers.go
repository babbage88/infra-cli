package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/babbage88/infra-cli/infractl_services"
	coredeploy "github.com/babbage88/infra-core/deployment"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type proxyCommandDefaults struct {
	Name        string
	PackageName string
	BinaryName  string
	ServiceName string
	ConfigPath  string
}

func init() {
	registerProxyInstallerCommand(proxyCommandDefaults{
		Name:        "nginx",
		PackageName: "nginx",
		BinaryName:  "nginx",
		ServiceName: "nginx",
		ConfigPath:  "/etc/nginx/nginx.conf",
	})

	registerProxyInstallerCommand(proxyCommandDefaults{
		Name:        "haproxy",
		PackageName: "haproxy",
		BinaryName:  "haproxy",
		ServiceName: "haproxy",
		ConfigPath:  "/etc/haproxy/haproxy.cfg",
	})

	registerProxyInstallerCommand(proxyCommandDefaults{
		Name:        "angie",
		PackageName: "angie",
		BinaryName:  "angie",
		ServiceName: "angie",
		ConfigPath:  "/etc/angie/angie.conf",
	})
}

func registerProxyInstallerCommand(defaults proxyCommandDefaults) {
	proxyViper := viper.New()

	cmd := &cobra.Command{
		Use:   defaults.Name,
		Short: fmt.Sprintf("Install and configure %s on a remote host over SSH", defaults.Name),
		Run: func(cmd *cobra.Command, args []string) {
			sshOpts, err := resolveRootSSHOptions("", "")
			if err != nil {
				slog.Error("Failed to resolve SSH options", "error", err.Error())
				os.Exit(1)
			}

			packageName := proxyViper.GetString("package_name")
			binaryName := proxyViper.GetString("binary_name")
			serviceName := proxyViper.GetString("service_name")
			configPath := proxyViper.GetString("config_path")
			localConfigPath := proxyViper.GetString("local_config_path")

			req := coredeploy.ProxyInstallRequest{
				SSH: coredeploy.SSHOptions{
					Host:       sshOpts.Host,
					User:       sshOpts.User,
					KeyPath:    sshOpts.KeyPath,
					Passphrase: sshOpts.Passphrase,
					UseAgent:   sshOpts.UseAgent,
					Port:       sshOpts.Port,
				},
				Name:            defaults.Name,
				PackageName:     packageName,
				BinaryName:      binaryName,
				ServiceName:     serviceName,
				ConfigPath:      configPath,
				LocalConfigPath: localConfigPath,
			}

			slog.Info(
				"Ensuring remote proxy is installed and configured",
				"proxy", defaults.Name,
				"host", sshOpts.Host,
				"service", serviceName,
				"config_path", configPath,
				"local_config_path", localConfigPath,
			)

			result, err := infractl_services.InstallProxy(req)
			if err != nil {
				slog.Error("Failed to configure proxy", "proxy", defaults.Name, "error", err.Error())
				os.Exit(1)
			}

			slog.Info(
				"Proxy installation and configuration completed",
				"proxy", defaults.Name,
				"host", sshOpts.Host,
				"service", serviceName,
				"config_path", configPath,
			)

			fmt.Printf("%s host: %s\n", result.Name, result.Host)
			fmt.Printf("%s package: %s\n", result.Name, result.PackageName)
			fmt.Printf("%s binary: %s\n", result.Name, result.BinaryName)
			fmt.Printf("%s service: %s\n", result.Name, result.ServiceName)
			fmt.Printf("%s config: %s\n", result.Name, result.ConfigPath)
			if result.LocalConfigPath != "" {
				fmt.Printf("%s local config: %s\n", result.Name, result.LocalConfigPath)
			}
		},
	}

	cmd.Flags().String("package-name", defaults.PackageName, fmt.Sprintf("Package name used to install %s", defaults.Name))
	cmd.Flags().String("binary-name", defaults.BinaryName, fmt.Sprintf("Binary name used to verify %s installation", defaults.Name))
	cmd.Flags().String("service-name", defaults.ServiceName, fmt.Sprintf("Service name used to manage %s", defaults.Name))
	cmd.Flags().String("config-path", defaults.ConfigPath, fmt.Sprintf("Remote config path for %s", defaults.Name))
	cmd.Flags().String("local-config", "", fmt.Sprintf("Optional local config file to upload to %s", defaults.Name))

	_ = proxyViper.BindPFlag("package_name", cmd.Flags().Lookup("package-name"))
	_ = proxyViper.BindPFlag("binary_name", cmd.Flags().Lookup("binary-name"))
	_ = proxyViper.BindPFlag("service_name", cmd.Flags().Lookup("service-name"))
	_ = proxyViper.BindPFlag("config_path", cmd.Flags().Lookup("config-path"))
	_ = proxyViper.BindPFlag("local_config_path", cmd.Flags().Lookup("local-config"))

	proxyCmd.AddCommand(cmd)
}

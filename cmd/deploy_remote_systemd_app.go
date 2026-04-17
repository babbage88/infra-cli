package cmd

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/babbage88/infra-cli/deployer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	defaultSystemdDir = "/etc/systemd/system"
)

var deployViper *viper.Viper

var deployCmd = &cobra.Command{
	Use:          "deploy",
	Short:        "Deploy a Go application as a remote systemd service",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveDeployFlags()
		if err != nil {
			return err
		}

		sshKey := expandPath(rootViperCfg.GetString("ssh_key"))
		if sshKey == "" {
			sshKey = defaultSSHKeyPath()
		}

		serviceAccount := map[int64]string{
			cfg.ServiceUid: cfg.ServiceUser,
		}

		appDeployer := deployer.NewRemoteSystemdDeployer(
			cfg.RemoteHostName,
			cfg.RemoteSshUser,
			cfg.AppName,
			cfg.SourceDir,
			deployer.WithEnvars(cfg.EnvVars),
			deployer.WithServiceAccount(serviceAccount),
			deployer.WithInstallDir(cfg.InstallDir),
			deployer.WithSystemdDir(cfg.SystemdDir),
			deployer.WithDestinationBin(cfg.DestinationBinary),
			deployer.WithSourceBin(cfg.SourceBin),
			deployer.WithSourceDir(cfg.SourceDir),
		)

		if err := appDeployer.StartSshDeploymentAgent(
			sshKey,
			rootViperCfg.GetString("ssh_passphrase"),
			rootViperCfg.GetBool("ssh_use_agent"),
			rootViperCfg.GetUint("ssh_port"),
		); err != nil {
			return fmt.Errorf("initialize ssh client: %w", err)
		}
		defer appDeployer.SshClient.SshClient.Close()

		slog.Info(
			"Ensuring remote systemd application is installed",
			"host", cfg.RemoteHostName,
			"ssh_user", cfg.RemoteSshUser,
			"app_name", cfg.AppName,
			"install_dir", cfg.InstallDir,
			"systemd_dir", cfg.SystemdDir,
			"source_bin", cfg.SourceBin,
			"destination_bin", cfg.DestinationBinary,
			"env_var_count", len(cfg.EnvVars),
		)

		return appDeployer.InstallApplication()
	},
}

// Struct for storing deployment flags
type DeployFlags struct {
	RemoteHostName    string            `mapstructure:"remote-host"`
	RemoteSshUser     string            `mapstructure:"remote-ssh-user"`
	AppName           string            `mapstructure:"app-name"`
	BinaryDir         string            `mapstructure:"binary-dir"`
	EnvVars           map[string]string `mapstructure:"env-vars"`
	EnvFile           string            `mapstructure:"env-file"`
	ServiceUser       string            `mapstructure:"service-user"`
	ServiceUid        int64             `mapstructure:"service-uid"`
	DestinationBinary string            `mapstructure:"dst-bin"`
	InstallDir        string            `mapstructure:"install-dir"`
	SystemdDir        string            `mapstructure:"systemd-dir"`
	SourceDir         string            `mapstructure:"source-dir"`
	SourceBin         string            `mapstructure:"source-bin"`
	SourceExcludes    []string          `mapstructure:"exclude-files"`
	RemoteDeployment  bool              `mapstructure:"remote-deployment"`
	DeployBinary      bool              `mapstructure:"deploy-binary"`
	VerboseLogging    bool              `mapstructure:"verbose"`
}

var deployFlags DeployFlags

func init() {
	deployViper = viper.New()

	rootCmd.AddCommand(deployCmd)

	deployCmd.Flags().StringVarP(&deployFlags.AppName, "app-name", "a", "", "Application name")
	deployCmd.Flags().StringToStringVar(&deployFlags.EnvVars, "env-vars", nil, "Environment variables to write to the remote env file")
	deployCmd.Flags().StringVar(&deployFlags.ServiceUser, "service-user", "", "User account that will run the service; defaults to app-name")
	deployCmd.Flags().Int64Var(&deployFlags.ServiceUid, "service-uid", 8888, "UID for the service account")
	deployCmd.Flags().StringVar(&deployFlags.DestinationBinary, "dst-bin", "", "Destination binary name on the remote host; defaults to app-name")
	deployCmd.Flags().StringVar(&deployFlags.InstallDir, "install-dir", "", "Remote install directory; defaults to /opt/<app-name>")
	deployCmd.Flags().StringVar(&deployFlags.EnvFile, "env-file", "", "Optional env file to merge into the remote service env file")
	deployCmd.Flags().StringVar(&deployFlags.SystemdDir, "systemd-dir", defaultSystemdDir, "Directory where systemd unit files are stored")
	deployCmd.Flags().StringVar(&deployFlags.SourceDir, "source-dir", ".", "Local source directory for the application")
	deployCmd.Flags().StringVar(&deployFlags.SourceBin, "source-bin", "", "Local binary to upload; defaults to app-name, resolved relative to source-dir when present")
	deployCmd.Flags().StringVar(&deployFlags.RemoteHostName, "remote-host", "", "Remote host to deploy to; defaults to global --ssh-remote-host")
	deployCmd.Flags().BoolVar(&deployFlags.RemoteDeployment, "remote-deployment", true, "Deprecated: remote deployment is always used by this command")
	deployCmd.Flags().BoolVar(&deployFlags.VerboseLogging, "verbose", true, "Verbose build logging")
	deployCmd.Flags().StringVar(&deployFlags.RemoteSshUser, "remote-ssh-user", "", "Remote SSH user to connect with; defaults to global --ssh-remote-user")
	deployCmd.Flags().StringSliceVar(&deployFlags.SourceExcludes, "exclude-files", nil, "Files to exclude during build")

	_ = deployViper.BindPFlags(deployCmd.Flags())
}

func resolveDeployFlags() (DeployFlags, error) {
	cfg := deployFlags

	if cfg.AppName == "" {
		return cfg, fmt.Errorf("--app-name is required")
	}
	if cfg.ServiceUid <= 0 {
		return cfg, fmt.Errorf("--service-uid must be greater than zero")
	}

	if cfg.RemoteHostName == "" {
		cfg.RemoteHostName = rootViperCfg.GetString("ssh_remote_host")
	}
	if cfg.RemoteHostName == "" {
		return cfg, fmt.Errorf("remote host is required; set --remote-host or the global --ssh-remote-host flag")
	}

	if cfg.RemoteSshUser == "" {
		cfg.RemoteSshUser = rootViperCfg.GetString("ssh_remote_user")
	}
	if cfg.RemoteSshUser == "" {
		cfg.RemoteSshUser = currentUserName()
	}

	if cfg.ServiceUser == "" {
		cfg.ServiceUser = cfg.AppName
	}
	if cfg.DestinationBinary == "" {
		cfg.DestinationBinary = cfg.AppName
	}
	if cfg.InstallDir == "" {
		cfg.InstallDir = filepath.ToSlash(filepath.Join("/opt", cfg.AppName))
	}
	if cfg.SystemdDir == "" {
		cfg.SystemdDir = defaultSystemdDir
	}
	if cfg.SourceDir == "" {
		cfg.SourceDir = "."
	}
	if cfg.SourceBin == "" {
		cfg.SourceBin = cfg.AppName
	}

	if cfg.EnvFile != "" {
		envFilePath := expandPath(cfg.EnvFile)
		envs, err := readEnvFile(envFilePath)
		if err != nil {
			return cfg, fmt.Errorf("read env file %q: %w", envFilePath, err)
		}
		cfg.EnvFile = envFilePath
		cfg.EnvVars = mergeStringMaps(envs, cfg.EnvVars)
	}
	if cfg.EnvVars == nil {
		cfg.EnvVars = map[string]string{}
	}

	cfg.SourceBin = resolveSourceBinPath(cfg.SourceDir, cfg.SourceBin)
	return cfg, nil
}

func resolveSourceBinPath(sourceDir, sourceBin string) string {
	if sourceBin == "" || filepath.IsAbs(sourceBin) {
		return sourceBin
	}

	candidate := filepath.Join(sourceDir, sourceBin)
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}

	return sourceBin
}

func mergeStringMaps(base, overrides map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(overrides))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range overrides {
		merged[key] = value
	}
	return merged
}

func formatEnvVars(envVars map[string]string) string {
	// Format environment variables for systemd unit file
	var formattedVars []string
	for key, value := range envVars {
		envLine := fmt.Sprintf(`Environment="%s=%s\n"`, key, value)
		formattedVars = append(formattedVars, envLine)
	}
	return fmt.Sprintf("%s", formattedVars)
}

func readEnvFile(filePath string) (map[string]string, error) {
	envMap := make(map[string]string)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		envMap[key] = value
	}

	if scanner.Err() != nil {
		return nil, scanner.Err()
	}

	return envMap, nil
}

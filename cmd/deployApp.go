package cmd

import (
	"fmt"
	"log/slog"
	"os/user"

	"github.com/babbage88/infra-cli/deployer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	deployUtilsPath           string = "remote_utils/bin"
	deployUtilsTar            string = "remote_utils.tar.gz"
	remoteUtilsPath           string = "/tmp/utils"
	validateUserUtilPath      string = "remote_utils/bin/validate-user"
	remoteValidateUserBaseCmd string = "/tmp/utils/remote_utils/validate-user"
	mkdirCmdBase              string = "mkdir"
	mkdirArgs                 string = "-p"
)

func getCurrentUserName() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}
	name := currentUser.Name
	return name, nil
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a Go web application as a systemd service",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Starting Cobra deploy command", "AppName", deployFlags.AppName)
		enVars := deployer.WithEnvars(deployFlags.EnvVars)
		serviceAccount := make(map[int64]string)
		serviceAccount[deployFlags.ServiceUid] = deployFlags.ServiceUser
		svcUser := deployer.WithServiceAccount(serviceAccount)
		appDeployer := deployer.NewRemoteSystemdDeployer(deployFlags.RemoteHostName,
			deployFlags.RemoteSshUser,
			deployFlags.AppName,
			deployFlags.SourceDir,
			enVars,
			svcUser,
			deployer.WithInstallDir(deployFlags.InstallDir),
			deployer.WithSystemdDir(deployFlags.SystemdDir),
		)
		err := appDeployer.StartSshDeploymentAgent(
			rootViperCfg.GetString("ssh_key"),
			rootViperCfg.GetString("ssh_passphrase"),
			deployFlags.EnvVars,
			rootViperCfg.GetBool("ssh_use_agent"),
			rootViperCfg.GetUint("ssh_port"),
		)
		defer appDeployer.SshClient.SshClient.Close()
		if err != nil {
			return fmt.Errorf("Error initializing ssh client %w", err)
		}
		slog.Info("Starting application installer", slog.String("RemoteHost", deployFlags.RemoteHostName), slog.String("AppName", deployFlags.AppName))
		err = appDeployer.InstallApplication()
		return err
	},
}

// Struct for storing deployment flags
type DeployFlags struct {
	RemoteHostName   string            `mapstructure:"remote-host"`
	RemoteSshUser    string            `mapstructure:"remote-ssh-user"`
	AppName          string            `mapstructure:"app-name"`
	BinaryDir        string            `mapstructure:"binary-dir"`
	EnvVars          map[string]string `mapstructure:"env-vars"`
	ServiceUser      string            `mapstructure:"service-user"`
	ServiceUid       int64             `mapstructure:"service-uid"`
	BinaryName       string            `mapstructure:"binary-name"`
	InstallDir       string            `mapstructure:"install-dir"`
	SystemdDir       string            `mapstructure:"systemd-dir"`
	SourceDir        string            `mapstructure:"source-dir"`
	SourceBin        string            `mapstructure:"source-bin"`
	SourceExcludes   []string          `mapstructure:"exclude-files"`
	RemoteDeployment bool              `mapstructure:"remote-deployment"`
	DeployBinary     bool              `mapstructure:"deploy-binary"`
	VerboseLogging   bool              `mapstructure:"verbose"`
}

var deployFlags DeployFlags

// init function to define the command flags and bind them with viper
func init() {
	curUser, _ := getCurrentUserName()

	rootCmd.AddCommand(deployCmd)

	// Define flags here
	deployCmd.Flags().StringVarP(&deployFlags.AppName, "app-name", "a", "", "The name of the application")
	deployCmd.Flags().StringToStringVar(&deployFlags.EnvVars, "env-vars", nil, "List of environment variables to set for the systemd service")
	deployCmd.Flags().StringVar(&deployFlags.ServiceUser, "service-user", "appuser", "User to run the service")
	deployCmd.Flags().Int64Var(&deployFlags.ServiceUid, "service-uid", 8888, "UID for service account to run the service")
	deployCmd.Flags().StringVar(&deployFlags.BinaryName, "binary-name", "smbplusplus", "Name of the compiled binary that will be output")
	deployCmd.Flags().StringVar(&deployFlags.InstallDir, "install-dir", "/etc/smbplusplus", "Directory to install the binary")
	deployCmd.Flags().StringVar(&deployFlags.SystemdDir, "systemd-dir", "/etc/systemd/system", "Directory where systemd service files will be stored")
	deployCmd.Flags().StringVar(&deployFlags.SourceDir, "source-dir", ".", "Source directory to build the application")
	deployCmd.Flags().StringVar(&deployFlags.SourceBin, "source-bin", ".", "Source Binary to install to build the application")
	deployCmd.Flags().StringVar(&deployFlags.RemoteHostName, "remote-host", ".", "Remote Hostname to deploy application to")
	deployCmd.Flags().BoolVar(&deployFlags.RemoteDeployment, "remote-deployment", true, "Select Remote destination Host, done via ssh.")
	deployCmd.Flags().BoolVar(&deployFlags.VerboseLogging, "verbose", true, "Verbose build logging.")
	deployCmd.Flags().StringVar(&deployFlags.RemoteSshUser, "remote-ssh-user", curUser, "Remote SSH user to connect with")
	deployCmd.Flags().StringSliceVar(&deployFlags.SourceExcludes, "exclude-files", nil, "Files to exclude durign build")

	// Bind the flags with viper
	viper.BindPFlags(deployCmd.Flags())
}

func formatEnvVars(envVars map[string]string) string {
	// Format environment variables for systemd unit file
	var formattedVars []string
	for key, value := range envVars {
		envLine := fmt.Sprintf(`Environment="%s=%s"`, key, value)
		formattedVars = append(formattedVars, envLine)
	}
	return fmt.Sprintf("%s", formattedVars)
}

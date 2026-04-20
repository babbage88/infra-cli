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

var garageDeployViper *viper.Viper

const defaultGarageVersion = "v2.2.0"

var storageS3DeployGarageNodeCmd = &cobra.Command{
	Use:   "deploy-garage-node",
	Short: "Install and configure a Garage S3 storage node on a remote host over SSH",
	Run: func(cmd *cobra.Command, args []string) {
		sshOpts, err := resolveRootSSHOptions("", "")
		if err != nil {
			slog.Error("Failed to resolve SSH options", "error", err.Error())
			os.Exit(1)
		}

		garageVersion := garageDeployViper.GetString("garage_version")
		garageBinaryPath := garageDeployViper.GetString("garage_binary_path")
		garageConfigPath := garageDeployViper.GetString("garage_config_path")
		garageMetadataDir := garageDeployViper.GetString("garage_metadata_dir")
		garageDataDir := garageDeployViper.GetString("garage_data_dir")
		garageReplicationFactor := garageDeployViper.GetInt("garage_replication_factor")
		garageDBEngine := garageDeployViper.GetString("garage_db_engine")
		garageRPCBindAddr := garageDeployViper.GetString("garage_rpc_bind_addr")
		garageRPCPublicAddr := garageDeployViper.GetString("garage_rpc_public_addr")
		garageRPCSecret := garageDeployViper.GetString("garage_rpc_secret")
		garageS3BindAddr := garageDeployViper.GetString("garage_s3_api_bind_addr")
		garageS3Region := garageDeployViper.GetString("garage_s3_region")
		garageS3RootDomain := garageDeployViper.GetString("garage_s3_root_domain")
		garageS3WebBindAddr := garageDeployViper.GetString("garage_s3_web_bind_addr")
		garageS3WebRootDomain := garageDeployViper.GetString("garage_s3_web_root_domain")
		garageS3WebIndex := garageDeployViper.GetString("garage_s3_web_index")
		garageK2VBindAddr := garageDeployViper.GetString("garage_k2v_api_bind_addr")
		garageAdminBindAddr := garageDeployViper.GetString("garage_admin_bind_addr")
		garageAdminToken := garageDeployViper.GetString("garage_admin_token")
		garageMetricsToken := garageDeployViper.GetString("garage_metrics_token")
		garageLogLevel := garageDeployViper.GetString("garage_log_level")

		req := coredeploy.GarageNodeRequest{
			SSH: coredeploy.SSHOptions{
				Host:       sshOpts.Host,
				User:       sshOpts.User,
				KeyPath:    sshOpts.KeyPath,
				Passphrase: sshOpts.Passphrase,
				UseAgent:   sshOpts.UseAgent,
				Port:       sshOpts.Port,
			},
			Version:           garageVersion,
			BinaryPath:        garageBinaryPath,
			ConfigPath:        garageConfigPath,
			MetadataDir:       garageMetadataDir,
			DataDir:           garageDataDir,
			DBEngine:          garageDBEngine,
			ReplicationFactor: garageReplicationFactor,
			RPCBindAddr:       garageRPCBindAddr,
			RPCPublicAddr:     garageRPCPublicAddr,
			RPCSecret:         garageRPCSecret,
			S3Region:          garageS3Region,
			S3APIBindAddr:     garageS3BindAddr,
			S3RootDomain:      garageS3RootDomain,
			S3WebBindAddr:     garageS3WebBindAddr,
			S3WebRootDomain:   garageS3WebRootDomain,
			S3WebIndex:        garageS3WebIndex,
			K2VAPIBindAddr:    garageK2VBindAddr,
			AdminAPIBindAddr:  garageAdminBindAddr,
			AdminToken:        garageAdminToken,
			MetricsToken:      garageMetricsToken,
			LogLevel:          garageLogLevel,
		}

		slog.Info(
			"Ensuring Garage is installed and configured",
			"host", sshOpts.Host,
			"rpc_public_addr", req.RPCPublicAddr,
			"s3_api_bind_addr", garageS3BindAddr,
			"admin_api_bind_addr", garageAdminBindAddr,
		)

		result, err := infractl_services.DeployGarageNode(req)
		if err != nil {
			slog.Error("Failed to configure Garage", "error", err.Error())
			os.Exit(1)
		}

		slog.Info(
			"Garage installation and node deployment completed",
			"host", result.Host,
			"config_path", result.ConfigPath,
			"rpc_public_addr", result.RPCPublicAddr,
		)

		fmt.Printf("Garage binary: %s\n", result.BinaryPath)
		fmt.Printf("Garage config: %s\n", result.ConfigPath)
		fmt.Printf("Garage service: %s\n", result.ServiceName)
		fmt.Printf("Garage RPC public address: %s\n", result.RPCPublicAddr)
		fmt.Printf("Garage S3 endpoint: %s\n", result.S3Endpoint)
		fmt.Printf("Garage admin endpoint: %s\n", result.AdminEndpoint)
		fmt.Printf("Garage admin token: %s\n", result.AdminToken)
		fmt.Printf("Garage metrics token: %s\n", result.MetricsToken)
	},
}

func init() {
	garageDeployViper = viper.New()

	storageS3Cmd.AddCommand(storageS3DeployGarageNodeCmd)

	storageS3DeployGarageNodeCmd.Flags().String("garage-version", defaultGarageVersion, "Garage version to install, for example v2.2.0")
	storageS3DeployGarageNodeCmd.Flags().String("garage-binary-path", "/usr/local/bin/garage", "Path where the Garage binary should be installed")
	storageS3DeployGarageNodeCmd.Flags().String("garage-config-path", "/etc/garage.toml", "Path for the Garage TOML configuration file")
	storageS3DeployGarageNodeCmd.Flags().String("garage-metadata-dir", "/var/lib/garage/meta", "Garage metadata directory")
	storageS3DeployGarageNodeCmd.Flags().String("garage-data-dir", "/var/lib/garage/data", "Garage data directory")
	storageS3DeployGarageNodeCmd.Flags().String("garage-db-engine", "sqlite", "Garage database engine")
	storageS3DeployGarageNodeCmd.Flags().Int("garage-replication-factor", 1, "Garage replication factor")
	storageS3DeployGarageNodeCmd.Flags().String("garage-rpc-bind-addr", "[::]:3901", "Garage RPC bind address")
	storageS3DeployGarageNodeCmd.Flags().String("garage-rpc-public-addr", "", "Garage RPC public address advertised to the cluster; defaults to <ssh-host>:3901")
	storageS3DeployGarageNodeCmd.Flags().String("garage-rpc-secret", "", "Shared Garage RPC secret; autogenerated when omitted")
	storageS3DeployGarageNodeCmd.Flags().String("garage-s3-api-bind-addr", "[::]:3900", "Garage S3 API bind address")
	storageS3DeployGarageNodeCmd.Flags().String("garage-s3-region", "garage", "Garage S3 region")
	storageS3DeployGarageNodeCmd.Flags().String("garage-s3-root-domain", ".s3.local", "Garage S3 API root domain")
	storageS3DeployGarageNodeCmd.Flags().String("garage-s3-web-bind-addr", "[::]:3902", "Garage S3 website bind address")
	storageS3DeployGarageNodeCmd.Flags().String("garage-s3-web-root-domain", ".web.local", "Garage S3 website root domain")
	storageS3DeployGarageNodeCmd.Flags().String("garage-s3-web-index", "index.html", "Garage S3 website index document")
	storageS3DeployGarageNodeCmd.Flags().String("garage-k2v-api-bind-addr", "[::]:3904", "Garage K2V API bind address")
	storageS3DeployGarageNodeCmd.Flags().String("garage-admin-bind-addr", "[::]:3903", "Garage admin API bind address")
	storageS3DeployGarageNodeCmd.Flags().String("garage-admin-token", "", "Garage admin API token; autogenerated when omitted")
	storageS3DeployGarageNodeCmd.Flags().String("garage-metrics-token", "", "Garage metrics token; autogenerated when omitted")
	storageS3DeployGarageNodeCmd.Flags().String("garage-log-level", "garage=info", "RUST_LOG value for the Garage systemd service")

	garageDeployViper.BindPFlag("garage_version", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-version"))
	garageDeployViper.BindPFlag("garage_binary_path", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-binary-path"))
	garageDeployViper.BindPFlag("garage_config_path", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-config-path"))
	garageDeployViper.BindPFlag("garage_metadata_dir", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-metadata-dir"))
	garageDeployViper.BindPFlag("garage_data_dir", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-data-dir"))
	garageDeployViper.BindPFlag("garage_db_engine", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-db-engine"))
	garageDeployViper.BindPFlag("garage_replication_factor", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-replication-factor"))
	garageDeployViper.BindPFlag("garage_rpc_bind_addr", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-rpc-bind-addr"))
	garageDeployViper.BindPFlag("garage_rpc_public_addr", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-rpc-public-addr"))
	garageDeployViper.BindPFlag("garage_rpc_secret", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-rpc-secret"))
	garageDeployViper.BindPFlag("garage_s3_api_bind_addr", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-s3-api-bind-addr"))
	garageDeployViper.BindPFlag("garage_s3_region", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-s3-region"))
	garageDeployViper.BindPFlag("garage_s3_root_domain", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-s3-root-domain"))
	garageDeployViper.BindPFlag("garage_s3_web_bind_addr", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-s3-web-bind-addr"))
	garageDeployViper.BindPFlag("garage_s3_web_root_domain", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-s3-web-root-domain"))
	garageDeployViper.BindPFlag("garage_s3_web_index", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-s3-web-index"))
	garageDeployViper.BindPFlag("garage_k2v_api_bind_addr", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-k2v-api-bind-addr"))
	garageDeployViper.BindPFlag("garage_admin_bind_addr", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-admin-bind-addr"))
	garageDeployViper.BindPFlag("garage_admin_token", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-admin-token"))
	garageDeployViper.BindPFlag("garage_metrics_token", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-metrics-token"))
	garageDeployViper.BindPFlag("garage_log_level", storageS3DeployGarageNodeCmd.Flags().Lookup("garage-log-level"))
}

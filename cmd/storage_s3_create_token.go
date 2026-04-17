package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/babbage88/infra-cli/deployer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var garageTokenViper *viper.Viper

var storageS3CreateTokenCmd = &cobra.Command{
	Use:   "create-token",
	Short: "Create an S3 access key and secret on a remote Garage instance over SSH",
	Run: func(cmd *cobra.Command, args []string) {
		sshHost := rootViperCfg.GetString("ssh_remote_host")
		sshUser := rootViperCfg.GetString("ssh_remote_user")
		sshKey := expandPath(rootViperCfg.GetString("ssh_key"))
		sshPassphrase := rootViperCfg.GetString("ssh_passphrase")
		useSshAgent := rootViperCfg.GetBool("ssh_use_agent")
		sshPort := rootViperCfg.GetUint("ssh_port")

		bucketName := garageTokenViper.GetString("garage_bucket_name")
		keyName := garageTokenViper.GetString("garage_key_name")
		createBucket := garageTokenViper.GetBool("garage_create_bucket")
		allowCreateBuckets := garageTokenViper.GetBool("garage_allow_create_buckets")
		allowRead := garageTokenViper.GetBool("garage_allow_read")
		allowWrite := garageTokenViper.GetBool("garage_allow_write")
		allowOwner := garageTokenViper.GetBool("garage_allow_owner")
		garageBinaryPath := garageTokenViper.GetString("garage_binary_path")
		garageConfigPath := garageTokenViper.GetString("garage_config_path")
		garageS3Endpoint := garageTokenViper.GetString("garage_s3_endpoint")
		garageLayoutZone := garageTokenViper.GetString("garage_layout_zone")
		garageLayoutCapacity := garageTokenViper.GetString("garage_layout_capacity")

		if sshHost == "" {
			slog.Error("SSH host is required", "hint", "set the global --ssh-remote-host flag")
			os.Exit(1)
		}
		if sshUser == "" {
			sshUser = currentUserName()
		}
		if sshKey == "" {
			sshKey = defaultSSHKeyPath()
		}
		if !cmd.Flags().Changed("bucket") && !allowCreateBuckets {
			bucketName = promptInput("Garage bucket name", bucketName)
		}
		if !cmd.Flags().Changed("key-name") {
			keyName = promptInput("Garage key name", keyName)
		}
		if keyName == "" {
			slog.Error("Key name is required")
			os.Exit(1)
		}

		if allowCreateBuckets && bucketName == "" &&
			!cmd.Flags().Changed("allow-read") &&
			!cmd.Flags().Changed("allow-write") &&
			!cmd.Flags().Changed("allow-owner") {
			allowRead = false
			allowWrite = false
			allowOwner = false
		}

		if bucketName == "" && !allowCreateBuckets {
			slog.Error("Bucket name is required unless --allow-create-buckets is set")
			os.Exit(1)
		}
		if garageS3Endpoint == "" {
			garageS3Endpoint = fmt.Sprintf("http://%s:3900", sshHost)
		}

		installer, err := deployer.NewRemoteGarageInstallerWithSsh(
			sshHost,
			sshUser,
			sshKey,
			sshPassphrase,
			useSshAgent,
			sshPort,
		)
		if err != nil {
			slog.Error(
				"Failed to initialize SSH client",
				"host", sshHost,
				"user", sshUser,
				"port", sshPort,
				"ssh_key", sshKey,
				"use_ssh_agent", useSshAgent,
				"error", err.Error(),
			)
			os.Exit(1)
		}
		defer installer.SshClient.Close()

		req := deployer.GarageTokenRequest{
			BucketName:         bucketName,
			KeyName:            keyName,
			CreateBucket:       createBucket,
			AllowCreateBuckets: allowCreateBuckets,
			AllowRead:          allowRead,
			AllowWrite:         allowWrite,
			AllowOwner:         allowOwner,
			BinaryPath:         garageBinaryPath,
			ConfigPath:         garageConfigPath,
			LayoutZone:         garageLayoutZone,
			LayoutCapacity:     garageLayoutCapacity,
		}

		slog.Info(
			"Creating Garage S3 credentials",
			"host", sshHost,
			"bucket", bucketName,
			"key_name", keyName,
			"create_bucket", createBucket,
			"allow_create_buckets", allowCreateBuckets,
		)

		creds, err := installer.CreateS3Token(req)
		if err != nil {
			slog.Error("Failed to create Garage S3 credentials", "error", err.Error())
			os.Exit(1)
		}

		fmt.Printf("S3 endpoint: %s\n", garageS3Endpoint)
		fmt.Printf("S3 bucket: %s\n", creds.BucketName)
		fmt.Printf("S3 access key: %s\n", creds.AccessKeyID)
		fmt.Printf("S3 secret key: %s\n", creds.SecretAccessKey)
		fmt.Printf("mc alias set garage %s %s %s\n", garageS3Endpoint, creds.AccessKeyID, creds.SecretAccessKey)
	},
}

func init() {
	garageTokenViper = viper.New()

	storageS3Cmd.AddCommand(storageS3CreateTokenCmd)

	storageS3CreateTokenCmd.Flags().String("bucket", "", "Garage bucket name")
	storageS3CreateTokenCmd.Flags().String("key-name", "", "Garage key name to create or fetch")
	storageS3CreateTokenCmd.Flags().Bool("create-bucket", true, "Create the Garage bucket if it does not exist")
	storageS3CreateTokenCmd.Flags().Bool("allow-create-buckets", false, "Grant the key permission to create new buckets across the Garage cluster")
	storageS3CreateTokenCmd.Flags().Bool("allow-read", true, "Grant read access to the bucket")
	storageS3CreateTokenCmd.Flags().Bool("allow-write", true, "Grant write access to the bucket")
	storageS3CreateTokenCmd.Flags().Bool("allow-owner", true, "Grant owner access to the bucket")
	storageS3CreateTokenCmd.Flags().String("garage-binary-path", "/usr/local/bin/garage", "Path to the Garage binary on the remote host")
	storageS3CreateTokenCmd.Flags().String("garage-config-path", "/etc/garage.toml", "Path to the Garage config on the remote host")
	storageS3CreateTokenCmd.Flags().String("s3-endpoint", "", "Advertised S3 endpoint to print with the generated credentials; defaults to http://<ssh-host>:3900")
	storageS3CreateTokenCmd.Flags().String("garage-layout-zone", "dc1", "Zone to use if Garage needs a single-node layout initialized")
	storageS3CreateTokenCmd.Flags().String("garage-layout-capacity", "100G", "Capacity to assign if Garage needs a single-node layout initialized")

	garageTokenViper.BindPFlag("garage_bucket_name", storageS3CreateTokenCmd.Flags().Lookup("bucket"))
	garageTokenViper.BindPFlag("garage_key_name", storageS3CreateTokenCmd.Flags().Lookup("key-name"))
	garageTokenViper.BindPFlag("garage_create_bucket", storageS3CreateTokenCmd.Flags().Lookup("create-bucket"))
	garageTokenViper.BindPFlag("garage_allow_create_buckets", storageS3CreateTokenCmd.Flags().Lookup("allow-create-buckets"))
	garageTokenViper.BindPFlag("garage_allow_read", storageS3CreateTokenCmd.Flags().Lookup("allow-read"))
	garageTokenViper.BindPFlag("garage_allow_write", storageS3CreateTokenCmd.Flags().Lookup("allow-write"))
	garageTokenViper.BindPFlag("garage_allow_owner", storageS3CreateTokenCmd.Flags().Lookup("allow-owner"))
	garageTokenViper.BindPFlag("garage_binary_path", storageS3CreateTokenCmd.Flags().Lookup("garage-binary-path"))
	garageTokenViper.BindPFlag("garage_config_path", storageS3CreateTokenCmd.Flags().Lookup("garage-config-path"))
	garageTokenViper.BindPFlag("garage_s3_endpoint", storageS3CreateTokenCmd.Flags().Lookup("s3-endpoint"))
	garageTokenViper.BindPFlag("garage_layout_zone", storageS3CreateTokenCmd.Flags().Lookup("garage-layout-zone"))
	garageTokenViper.BindPFlag("garage_layout_capacity", storageS3CreateTokenCmd.Flags().Lookup("garage-layout-capacity"))
}

/*
Copyright © 2025 Justin Trahan <justin@trahan.dev>
*/
package cmd

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/babbage88/infra-cli/internal/files"
	"github.com/babbage88/infra-cli/ssh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func compressFiles(src string, tarOutputPath string) {

	log.Printf("Creating tar.gz archive from %s -> %s", src, tarOutputPath)

	outFile, err := os.Create(tarOutputPath)
	if err != nil {
		log.Fatalf("Could not create output file: %v", err)
	}
	defer outFile.Close()

	err = files.TarAndGzipFiles(src, outFile)
	if err != nil {
		log.Fatalf("Error creating tar.gz: %v", err)
	}

	log.Printf("Archive created successfully at %s", tarOutputPath)
}

// metaCmd represents the meta command
var metaCmd = &cobra.Command{
	Use:   "meta",
	Short: "Debugging/Development subcommand for Viper/Cobra",
	Long:  `Subcommand for debugging this Cobra/Viper application`,
	Run: func(cmd *cobra.Command, args []string) {
		srcFiles := viper.GetString("meta_src")
		tarOutputPath := viper.GetString("meta_tar_output")

		shost := rootViperCfg.GetString("ssh_remote_host")
		suser := rootViperCfg.GetString("ssh_remote_user")
		skeypath := rootViperCfg.GetString("ssh_key")
		sport := rootViperCfg.GetUint("ssh_port")
		suseagent := rootViperCfg.GetBool("ssh_use_agent")
		skeypass := rootViperCfg.GetString("ssh_passphrase")
		src := viper.GetString("meta_src")
		dst := viper.GetString("meta_dst")
		scmd := viper.GetString("meta_sshcmd")

		compressFiles(srcFiles, tarOutputPath)

		slog.Info("Creating new SSH client connnection host:, user: ssh-key: \n", "Host", shost, "User", suser, "ssh-key", skeypath)

		rclient, err := ssh.NewRemoteAppDeploymentAgentWithSshKey(shost,
			suser, src,
			dst, skeypath, skeypass,
			nil, suseagent, sport)

		defer rclient.SshClient.Close()

		if err != nil {
			slog.Error("SSH error", "error", err.Error())
		}
		rclient.UploadBin(tarOutputPath, fmt.Sprint("/tmp/", tarOutputPath))
		if err != nil {
			slog.Error("Error Uploading files", "error", err.Error())
		}

		baseCommand := parseBaseCommand(scmd)
		cmdArgs := parseCmdStringArgsToSlice(scmd)
		err = rclient.RunCommandAndGetOutput(baseCommand, cmdArgs)
		if err != nil {
			slog.Info("err executing command", "error", err.Error())
		}
		slog.Info("Success executin upload and command", "files", tarOutputPath, "command", scmd)
	},
}

func init() {
	rootCmd.AddCommand(metaCmd)
	metaCmd.PersistentFlags().StringP("meta-src", "x", "remote_utils/bin/validate-user",
		"Source files to upload")
	metaCmd.PersistentFlags().StringP("meta-dst", "y", "/tmp/validate-user",
		"Destination on remote hostfor remote file copy")
	metaCmd.PersistentFlags().StringP("meta-sshcmd", "z", "whoami",
		"Remote command to run")
	metaCmd.PersistentFlags().StringVar(&metaCfgFile, "meta-config", "",
		"Config file (default is meta-config.yaml)")
	metaCmd.PersistentFlags().String("meta-tar-output", "output.tar.gz", "Path to write tar.gz archive")

	if metaCfgFile != "" {
		err := mergeMetaConfigFile()
		if err != nil {
			slog.Error("error merging meta-config", "error", err.Error())
		}

		err = loadRootConfigFile()
		if err != nil {
			slog.Error("error merging meta-config", "error", err.Error())
		}
	}

	viper.BindPFlag("meta_src", metaCmd.PersistentFlags().Lookup("meta-src"))
	viper.BindPFlag("meta_dst", metaCmd.PersistentFlags().Lookup("meta-dst"))
	viper.BindPFlag("meta_sshcmd", metaCmd.PersistentFlags().Lookup("meta-sshcmd"))
	viper.BindPFlag("meta_tar_output", metaCmd.PersistentFlags().Lookup("meta-tar-output"))

	cobra.OnInitialize(func() {
		err := mergeMetaConfigFile()
		if err != nil {
			slog.Error("error merging meta-config", "error", err.Error())
		}

		err = loadRootConfigFile()
		if err != nil {
			slog.Error("error merging meta-config", "error", err.Error())
		}
	})
}

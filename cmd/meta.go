/*
Copyright © 2025 Justin Trahan <justin@trahan.dev>
*/
package cmd

import (
	"log/slog"

	"github.com/babbage88/infra-cli/internal/files"
	"github.com/babbage88/infra-cli/ssh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func compressFiles(src string, tarOutputPath string, excludes []string) {
	slog.Info("Creating tar.gz archive from", "src", src, "tarOutputPath", tarOutputPath)

	err := files.CreateTarGzWithExcludes(src, tarOutputPath, excludes)
	if err != nil {
		slog.Error("Error creating tar.gz", "error", err.Error())
	}

	slog.Info("Archive created successfully", "tarOutputPath", tarOutputPath)
}

// metaCmd represents the meta command
var metaCmd = &cobra.Command{
	Use:   "meta",
	Short: "Debugging/Development subcommand for Viper/Cobra",
	Long:  `Subcommand for debugging this Cobra/Viper application`,
	Run: func(cmd *cobra.Command, args []string) {
		tarOutputPath := viper.GetString("meta_tar_output")
		exctractDir := viper.GetString("meta_extract_dir")
		shost := rootViperCfg.GetString("ssh_remote_host")
		suser := rootViperCfg.GetString("ssh_remote_user")
		skeypath := rootViperCfg.GetString("ssh_key")
		sport := rootViperCfg.GetUint("ssh_port")
		suseagent := rootViperCfg.GetBool("ssh_use_agent")
		skeypass := rootViperCfg.GetString("ssh_passphrase")
		src := viper.GetString("meta_src")
		dst := viper.GetString("meta_dst")
		scmd := viper.GetString("meta_sshcmd")
		extractCmdMap := make(map[string][]string)

		//parentDir := filepath.Dir(exctractDir)
		preExtractCmd := []string{"mkdir", "-p", exctractDir}
		extractCmd := []string{"tar", "-xvzf", tarOutputPath, "-C", exctractDir}

		extractCmdMap[preExtractCmd[0]] = preExtractCmd[1:]
		extractCmdMap[extractCmd[0]] = extractCmd[1:]

		baseCommand := parseBaseCommand(scmd)
		cmdArgs := parseCmdStringArgsToSlice(scmd)

		compressFiles(src, tarOutputPath, []string{"vendor"})

		slog.Info("Creating new SSH client connnection host:, user: ssh-key: \n", "Host", shost, "User", suser, "ssh-key", skeypath)

		rclient, err := ssh.NewRemoteAppDeploymentAgentWithSshKey(shost,
			suser, src,
			dst, skeypath, skeypass,
			nil, suseagent, sport)

		defer rclient.SshClient.Close()

		if err != nil {
			slog.Error("SSH error", "error", err.Error())
		}

		err = rclient.UploadBin(tarOutputPath, tarOutputPath)
		if err != nil {
			slog.Error("Error Uploading files", "error", err.Error())
		}

		for k, v := range extractCmdMap {
			slog.Info("running extract commnds", "cmd", k, "args", v)
			err = rclient.RunCommandAndGetOutput(k, v)
			if err != nil {
				slog.Error("Error running cmd", "error", err.Error())
			}
		}

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
	metaCmd.PersistentFlags().String("meta-extract-dir", "/tmp/utils", "Path on remote host to extract tar.gz")
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
	viper.BindPFlag("meta_extract_dir", metaCmd.PersistentFlags().Lookup("meta-extract-dir"))

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

/*
Copyright © 2025 Justin Trahan <justin@trahan.dev>
*/
package cmd

import (
	"log"

	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/babbage88/infra-cli/ssh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// metaCmd represents the meta command
var metaCmd = &cobra.Command{
	Use:   "meta",
	Short: "Debugging/Development subcommand for Viper/Cobra",
	Long:  `Subcommand for debugging this Cobra/Viper application`,
	Run: func(cmd *cobra.Command, args []string) {
		shost := rootViperCfg.GetString("ssh_remote_host")
		suser := rootViperCfg.GetString("ssh_remote_user")
		skeypath := rootViperCfg.GetString("ssh_key")
		sport := rootViperCfg.GetUint("ssh_port")
		suseagent := rootViperCfg.GetBool("ssh_use_agent")
		skeypass := rootViperCfg.GetString("ssh_passphrase")
		src := viper.GetString("meta_src")
		dst := viper.GetString("meta_dst")
		scmd := viper.GetString("meta_sshcmd")

		log.Printf("Creating new SSH client connnection host: %s, user: %s ssh-key: %s\n", shost, suser, skeypath)

		rclient, err := ssh.NewRemoteAppDeploymentAgentWithSshKey(shost,
			suser, src,
			dst, skeypath, skeypass,
			nil, suseagent, sport)

		defer rclient.SshClient.Close()

		if err != nil {
			log.Fatalf("ssh errore: %s\n", err.Error())
		}
		rclient.UploadBin("remote_utils/bin/validate-user", "/tmp/validate-user")
		if err != nil {
			log.Printf("Error uploading files\n")
		}

		baseCommand := parseBaseCommand(scmd)
		cmdArgs := parseCmdStringArgsToSlice(scmd)
		err = rclient.RunCommandAndGetOutput(baseCommand, cmdArgs)
		if err != nil {
			log.Printf("cmd err: %s\n", err.Error())
		}
		log.Println("ran command")
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
	if metaCfgFile != "" {
		err := mergeMetaConfigFile()
		if err != nil {
			pretty.PrintErrorf("error merging meta-config %s", err.Error())
		}

		err = loadRootConfigFile()
		if err != nil {
			pretty.PrintErrorf("error merging meta-config %s", err.Error())
		}
	}

	viper.BindPFlag("meta_src", metaCmd.PersistentFlags().Lookup("meta-src"))
	viper.BindPFlag("meta_dst", metaCmd.PersistentFlags().Lookup("meta-dst"))
	viper.BindPFlag("meta_sshcmd", metaCmd.PersistentFlags().Lookup("meta-sshcmd"))

	cobra.OnInitialize(func() {
		err := mergeMetaConfigFile()
		if err != nil {
			pretty.PrintErrorf("error merging meta-config %s", err.Error())
		}

		err = loadRootConfigFile()
		if err != nil {
			pretty.PrintErrorf("error merging meta-config %s", err.Error())
		}
	})
}

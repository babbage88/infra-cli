/*
Copyright © 2025 Justin Trahan <justin@trahan.dev>
*/
package cmd

import (
	"log"

	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/babbage88/infra-cli/remote_utils/ssh"
	"github.com/spf13/cobra"
)

// metaCmd represents the meta command
var metaCmd = &cobra.Command{
	Use:   "meta",
	Short: "Debugging/Development subcommand for Viper/Cobra",
	Long:  `Subcommand for debugging this Cobra/Viper application`,
	Run: func(cmd *cobra.Command, args []string) {
		rclient, err := ssh.NewRemoteAppDeploymentAgentWithSshKey("trahdev2", "jtrahan", "remote_utils/bin", "/tmp", rootViperCfg.GetString("ssh_key"), rootViperCfg.GetString("ssh_passphrase"), nil)
		if err != nil {
			log.Fatalf("ssh errore: %s\n", err.Error())
		}
		err = rclient.RunCommand("ls", []string{"-la"})
		if err != nil {
			log.Printf("cmd err: %s\n", err.Error())
		}
		log.Println("ran command")
	},
}

func init() {
	rootCmd.AddCommand(metaCmd)
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

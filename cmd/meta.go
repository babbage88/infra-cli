/*
Copyright © 2025 Justin Trahan <justin@trahan.dev>
*/
package cmd

import (
	"fmt"
	"time"

	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// metaCmd represents the meta command
var metaCmd = &cobra.Command{
	Use:   "meta",
	Short: "Debugging/Development subcommand for Viper/Cobra",
	Long:  `Subcommand for debugging this Cobra/Viper application`,
	Run: func(cmd *cobra.Command, args []string) {
		var recordsBatch DnsRecordBatchRequest
		if err := metaCfg.Unmarshal(&recordsBatch); err != nil {
			pretty.PrintErrorf("Unable to decode into struct: %v", err)
		}
		for _, record := range recordsBatch.Records {
			pretty.Printf("ZoneName: %s", record.ZoneName)
			pretty.Printf("Name: %s", record.Name)
			pretty.Printf("Content: %s", record.Content)
		}
		vkeys := rootCfg.AllKeys()
		if metaCfgFile != "" {
			mkeys := metaCfg.AllKeys()
			cfgUsed := metaCfg.ConfigFileUsed()
			pretty.Printf("[DEBUG] - %s | %s", pretty.DateTimeSting(time.Now()), cfgUsed)
			for _, key := range mkeys {
				pretty.Printf("key: %s value: %s\n", key, viper.GetString(key))
				var recordsBatch DnsRecordBatchRequest
				if err := viper.Unmarshal(&recordsBatch); err != nil {
					pretty.PrintErrorf("Unable to decode into struct: %v", err)
				}
				for _, record := range recordsBatch.Records {
					pretty.Printf("ZoneName: %s", record.ZoneName)
					pretty.Printf("Name: %s", record.Name)
					pretty.Printf("Content: %s", record.Content)
				}
			}

		}

		pretty.Printf("[DEBUG] - %s | Main meta Exec func\n", pretty.DateTimeSting(time.Now()))
		msg := fmt.Sprintf("AllKeys: Total: %d\n", len(vkeys))
		pretty.Print(msg)

		for _, key := range vkeys {
			pretty.Printf("key: %s value: %s\n", key, viper.GetString(key))
		}
		pretty.Printf("Total: %d", len(vkeys))
	},
}

func init() {
	rootCmd.AddCommand(metaCmd)
	metaCmd.PersistentFlags().StringVar(&metaCfgFile, "meta-config", "",
		"Config file (default is meta-config.yaml)")
	err := mergeMetaConfigFile()
	if err != nil {
		pretty.PrintErrorf("error merging meta-config %s", err.Error())
	}

	err = loadRootConfigFile()
	if err != nil {
		pretty.PrintErrorf("error merging meta-config %s", err.Error())
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

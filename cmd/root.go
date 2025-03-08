package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "infractl",
	Short: "Infractl - Manage the universe for you applications",
	Long:  `Commands and utilities for managing a go-infra instance or it's child applications.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Load default config values
	rootCmd.PersistentFlags().String("cfapitoken", viper.GetString("cfapitoken"), "Cloudflare API Token")
	viper.BindPFlag("cfapitoken", rootCmd.PersistentFlags().Lookup("cfapitoken"))
}

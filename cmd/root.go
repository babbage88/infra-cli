package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	jwtSecret string
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
	// Read Viper config before execution
	cobra.OnInitialize(func() {
		jwtKeyName = viper.GetString("jwt_key_name")
		jwtTokenName = viper.GetString("jwt_token_name")
		jwtSecret = viper.GetString("jwt_secret")
	})

}

func writeToEnvFile(filename, key, value string) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error writing to .env file:", err)
		return
	}
	defer f.Close()

	_, err = f.WriteString(fmt.Sprintf("%s=%s\n", key, value))
	if err != nil {
		fmt.Println("Error writing to .env file:", err)
	}
}

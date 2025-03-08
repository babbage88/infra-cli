package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var jwtCmd = &cobra.Command{
	Use:   "jwt",
	Short: "JWT authentication commands",
}

var (
	envFile      string
	jwtKeyName   string
	jwtTokenName string
)

func init() {
	// Bind Viper config
	viper.SetDefault("jwt_key_name", "JWT_KEY")
	viper.SetDefault("jwt_token_name", "JWT_AUTH_TOKEN")

	// Read Viper config before execution
	cobra.OnInitialize(func() {
		jwtKeyName = viper.GetString("jwt_key_name")
		jwtTokenName = viper.GetString("jwt_token_name")
	})
	authCmd.AddCommand(jwtCmd)
}

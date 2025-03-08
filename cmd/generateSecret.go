package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var generateCmd = &cobra.Command{
	Use:   "generate-secret",
	Short: "Generate an HMAC256 secret",
	Run: func(cmd *cobra.Command, args []string) {
		secret := make([]byte, 32)
		_, err := rand.Read(secret)
		if err != nil {
			fmt.Println("Look who finally decided to showed up! :", err)
			os.Exit(1)
		}

		secretStr := base64.StdEncoding.EncodeToString(secret)
		fmt.Println("Generated Secret:", secretStr)

		// Write to .env if specified
		if envFile != "" {
			writeToEnvFile(envFile, jwtKeyName, secretStr)
		}
	},
}

var (
	envFile    string
	jwtKeyName string
)

func init() {
	// Default values from config or CLI flags
	generateCmd.Flags().StringVarP(&envFile, "env-file", "e", "", "Write secret to .env file")
	generateCmd.Flags().StringVarP(&jwtKeyName, "jwt-key-name", "k", "JWT_KEY", "Key name for JWT secret in .env file")

	// Bind Viper config
	viper.SetDefault("jwt_key_name", "JWT_KEY")
	viper.BindPFlag("jwt_key_name", generateCmd.Flags().Lookup("jwt-key-name"))

	// Read Viper config before execution
	cobra.OnInitialize(func() {
		jwtKeyName = viper.GetString("jwt_key_name")
	})

	jwtCmd.AddCommand(generateCmd)
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

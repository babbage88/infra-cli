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
		length := viper.GetInt64("secret_length")
		secret := make([]byte, length)
		_, err := rand.Read(secret)
		if err != nil {
			fmt.Println("Look who finally decided to showed up! :", err)
			os.Exit(1)
		}

		secretStr := base64.StdEncoding.EncodeToString(secret)
		if !rawFlag {
			fmt.Printf("JWT Signing Secret: %s\n", secretStr)
		} else {
			fmt.Printf("%s\n", secretStr)
		}

		// Write to .env if specified
		if envFile != "" {
			writeToEnvFile(envFile, jwtKeyName, secretStr)
		}

		if yamlFile != "" {
			writeToYAML(yamlFile, jwtKeyName, secretStr)
		}

		if jsonFile != "" {
			writeToJSON(jsonFile, jwtKeyName, secretStr)
		}

		if tomlFile != "" {
			writeToTOML(tomlFile, jwtKeyName, secretStr)
		}

	},
}

func init() {
	// Default values from config or CLI flags
	generateCmd.Flags().StringVarP(&yamlFile, "output-yaml-file", "y", "", "Write secret to .yaml file")
	generateCmd.Flags().StringVarP(&jsonFile, "output-json-file", "j", "", "Write secret to .json file")
	generateCmd.Flags().StringVarP(&tomlFile, "output-toml-file", "t", "", "Write secret to .toml file")
	generateCmd.Flags().StringVarP(&envFile, "output-env-file", "e", "", "Write secret to .env file")
	generateCmd.Flags().StringVar(&jwtKeyName, "jwt-key-name", "JWT_KEY", "Key name for JWT secret in .env file")
	generateCmd.Flags().StringVarP(&jwtTokenName, "jwt-token-name", "a", "JWT_AUTH_TOKEN", "Key name for JWT tokens in .env file")
	generateCmd.Flags().Int64("secret-length", 64, "The number of random characters to use for the secrest's length")

	// Bind Viper config
	viper.SetDefault("jwt_key_name", "JWT_KEY")
	viper.SetDefault("jwt_token_name", "JWT_AUTH_TOKEN")
	viper.SetDefault("secret_length", int64(64))
	viper.BindPFlag("secret_length", generateCmd.Flags().Lookup("secret-length"))
	viper.BindPFlag("jwt_key_name", generateCmd.Flags().Lookup("jwt-key-name"))
	viper.BindPFlag("jwt_token_name", generateCmd.Flags().Lookup("jwt-token-name"))
	viper.BindPFlag("output_yaml_file", generateCmd.Flags().Lookup("output-yaml-file"))
	viper.BindPFlag("output_json_file", generateCmd.Flags().Lookup("output-json-file"))
	viper.BindPFlag("output_toml_file", generateCmd.Flags().Lookup("output-toml-file"))

	// Read Viper config before execution
	cobra.OnInitialize(func() {
		jwtKeyName = viper.GetString("jwt_key_name")
		jwtTokenName = viper.GetString("jwt_token_name")
	})

	jwtCmd.AddCommand(generateCmd)
}

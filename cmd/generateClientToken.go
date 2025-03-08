package cmd

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var secret string

var newClientTokenCmd = &cobra.Command{
	Use:   "new-client-token",
	Short: "Generate a new JWT client token",
	Run: func(cmd *cobra.Command, args []string) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"exp": time.Now().Add(time.Hour * 24).Unix(),
		})

		tokenString, err := token.SignedString([]byte(secret))
		if err != nil {
			fmt.Println("Error generating token:", err)
			return
		}

		fmt.Println("JWT Token:", tokenString)

		// Write to .env if specified
		if envFile != "" {
			writeToEnvFile(envFile, jwtTokenName, tokenString)
		}

		if yamlFile != "" {
			writeToYAML(yamlFile, jwtTokenName, tokenString)
		}

		if jsonFile != "" {
			writeToJSON(yamlFile, jwtTokenName, tokenString)
		}

		if tomlFile != "" {
			writeToTOML(yamlFile, jwtTokenName, tokenString)
		}
	},
}

func init() {
	newClientTokenCmd.Flags().StringVarP(&secret, "secret", "s", "", "Secret key for signing the JWT")
	newClientTokenCmd.Flags().StringVarP(&envFile, "output-env-file", "o", "", "Write token to .env file")
	newClientTokenCmd.Flags().StringVarP(&yamlFile, "output-yaml-file", "y", "", "Write token to .yaml file")
	newClientTokenCmd.Flags().StringVarP(&yamlFile, "output-json-file", "j", "", "Write token to .json file")
	newClientTokenCmd.Flags().StringVarP(&yamlFile, "output-toml-file", "t", "", "Write token to .toml file")
	newClientTokenCmd.Flags().StringVarP(&jwtKeyName, "jwt-key-name", "k", "JWT_KEY", "Key name for JWT secret in .env file")
	newClientTokenCmd.Flags().StringVarP(&jwtTokenName, "jwt-token-name", "a", "JWT_AUTH_TOKEN", "Key name for JWT tokens in .env file")

	viper.BindPFlag("jwt_key_name", newClientTokenCmd.Flags().Lookup("jwt-key-name"))
	viper.BindPFlag("jwt_token_name", newClientTokenCmd.Flags().Lookup("jwt-token-name"))
	viper.BindPFlag("jwt_token", newClientTokenCmd.Flags().Lookup("secret"))
	viper.BindPFlag("output_yaml_file", newClientTokenCmd.Flags().Lookup("output-yaml-file"))
	viper.BindPFlag("output_json_file", newClientTokenCmd.Flags().Lookup("output-json-file"))
	viper.BindPFlag("output_toml_file", newClientTokenCmd.Flags().Lookup("output-toml-file"))

	jwtCmd.AddCommand(newClientTokenCmd)
}

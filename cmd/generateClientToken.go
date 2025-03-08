package cmd

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cobra"
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
			writeToEnvFile(envFile, "JWT_TOKEN", tokenString)
		}
	},
}

func init() {
	newClientTokenCmd.Flags().StringVarP(&secret, "secret", "s", "", "Secret key for signing the JWT")
	newClientTokenCmd.MarkFlagRequired("secret")
	newClientTokenCmd.Flags().StringVarP(&envFile, "env-file", "e", "", "Write token to .env file")
	jwtCmd.AddCommand(newClientTokenCmd)
}

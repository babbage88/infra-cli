package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate-secret",
	Short: "Generate an HMAC256 secret",
	Run: func(cmd *cobra.Command, args []string) {
		secret := make([]byte, 32)
		_, err := rand.Read(secret)
		if err != nil {
			fmt.Println("Error generating secret:", err)
			os.Exit(1)
		}

		secretStr := base64.StdEncoding.EncodeToString(secret)
		fmt.Println("Generated Secret:", secretStr)

		// Write to .env if specified
		if envFile != "" {
			writeToEnvFile(envFile, "JWT_KEY", secretStr)
		}
	},
}

var envFile string

func init() {
	generateCmd.Flags().StringVarP(&envFile, "output-env-file", "e", "", "Write secret to .env file")
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

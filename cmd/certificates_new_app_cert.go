package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/babbage88/infra-cli/deployer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var certificatesAppViper *viper.Viper

var certificatesNewAppCertCmd = &cobra.Command{
	Use:   "new-app-cert",
	Short: "Generate an application certificate signed by the local development CA",
	Run: func(cmd *cobra.Command, args []string) {
		outputDir := expandPath(certificatesAppViper.GetString("cert_output_dir"))
		caName := certificatesAppViper.GetString("ca_name")
		appName := certificatesAppViper.GetString("cert_app_name")
		commonName := certificatesAppViper.GetString("cert_common_name")
		organization := certificatesAppViper.GetString("cert_organization")
		sans := certificatesAppViper.GetStringSlice("cert_sans")
		yearsValid := certificatesAppViper.GetInt("cert_years_valid")

		if !cmd.Flags().Changed("app-name") {
			appName = promptInput("Application certificate name", appName)
		}
		if appName == "" {
			slog.Error("Application certificate name is required")
			os.Exit(1)
		}

		req := deployer.GenerateAppCertRequest{
			OutputDir:    outputDir,
			CAName:       caName,
			AppName:      appName,
			CommonName:   commonName,
			Organization: organization,
			SANs:         sans,
			YearsValid:   yearsValid,
		}

		slog.Info("Generating application certificate", "output_dir", outputDir, "app_name", appName)

		result, err := deployer.GenerateAppCertificate(req)
		if err != nil {
			slog.Error("Failed to generate application certificate", "error", err.Error())
			os.Exit(1)
		}

		fmt.Printf("Certificate: %s\n", result.CertPath)
		fmt.Printf("Private key: %s\n", result.KeyPath)
		fmt.Printf("CA certificate: %s\n", result.CACertPath)
	},
}

func init() {
	certificatesAppViper = viper.New()

	certificatesCmd.AddCommand(certificatesNewAppCertCmd)

	certificatesNewAppCertCmd.Flags().String("output-dir", "./certs", "Directory where generated certificates should be written")
	certificatesNewAppCertCmd.Flags().String("ca-name", "infractl-dev-ca", "Logical name for the CA to sign the app certificate")
	certificatesNewAppCertCmd.Flags().String("app-name", "", "Application certificate name used in output filenames")
	certificatesNewAppCertCmd.Flags().String("common-name", "localhost", "Application certificate common name")
	certificatesNewAppCertCmd.Flags().String("organization", "Infractl Development", "Application certificate organization name")
	certificatesNewAppCertCmd.Flags().StringSlice("san", []string{"localhost", "127.0.0.1", "::1"}, "SAN entries to include in the application certificate")
	certificatesNewAppCertCmd.Flags().Int("years-valid", 2, "Number of years the generated application certificate should remain valid")

	certificatesAppViper.BindPFlag("cert_output_dir", certificatesNewAppCertCmd.Flags().Lookup("output-dir"))
	certificatesAppViper.BindPFlag("ca_name", certificatesNewAppCertCmd.Flags().Lookup("ca-name"))
	certificatesAppViper.BindPFlag("cert_app_name", certificatesNewAppCertCmd.Flags().Lookup("app-name"))
	certificatesAppViper.BindPFlag("cert_common_name", certificatesNewAppCertCmd.Flags().Lookup("common-name"))
	certificatesAppViper.BindPFlag("cert_organization", certificatesNewAppCertCmd.Flags().Lookup("organization"))
	certificatesAppViper.BindPFlag("cert_sans", certificatesNewAppCertCmd.Flags().Lookup("san"))
	certificatesAppViper.BindPFlag("cert_years_valid", certificatesNewAppCertCmd.Flags().Lookup("years-valid"))
}

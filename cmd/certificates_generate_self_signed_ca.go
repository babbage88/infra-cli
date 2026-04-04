package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/babbage88/infra-cli/deployer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var certificatesCAViper *viper.Viper

var certificatesGenerateSelfSignedCACmd = &cobra.Command{
	Use:   "generate-self-signed-ca",
	Short: "Generate a local development CA certificate and trust it locally and optionally on an SSH target",
	Run: func(cmd *cobra.Command, args []string) {
		outputDir := expandPath(certificatesCAViper.GetString("cert_output_dir"))
		caName := certificatesCAViper.GetString("ca_name")
		commonName := certificatesCAViper.GetString("ca_common_name")
		organization := certificatesCAViper.GetString("ca_organization")
		sans := certificatesCAViper.GetStringSlice("ca_sans")
		yearsValid := certificatesCAViper.GetInt("cert_years_valid")

		sshHost := rootViperCfg.GetString("ssh_remote_host")
		sshUser := rootViperCfg.GetString("ssh_remote_user")
		sshKey := expandPath(rootViperCfg.GetString("ssh_key"))
		sshPassphrase := rootViperCfg.GetString("ssh_passphrase")
		useSshAgent := rootViperCfg.GetBool("ssh_use_agent")
		sshPort := rootViperCfg.GetUint("ssh_port")

		req := deployer.GenerateCARequest{
			OutputDir:    outputDir,
			CAName:       caName,
			CommonName:   commonName,
			Organization: organization,
			SANs:         sans,
			YearsValid:   yearsValid,
		}

		slog.Info("Generating development CA certificate", "output_dir", outputDir, "ca_name", caName)

		result, err := deployer.GenerateAndTrustLocalCA(req)
		if err != nil {
			slog.Error("Failed to generate or trust local CA", "error", err.Error())
			os.Exit(1)
		}

		if sshHost != "" {
			if sshUser == "" {
				sshUser = currentUserName()
			}
			if sshKey == "" {
				sshKey = defaultSSHKeyPath()
			}

			sshReq := deployer.RemoteTrustCARequest{
				Hostname:   sshHost,
				Username:   sshUser,
				SSHKey:     sshKey,
				Passphrase: sshPassphrase,
				UseAgent:   useSshAgent,
				Port:       sshPort,
				CertPath:   result.CertPath,
				CAName:     caName,
			}

			if err := deployer.TrustCARemoteViaSSH(sshReq); err != nil {
				slog.Error("Failed to trust CA on remote host", "host", sshHost, "error", err.Error())
				os.Exit(1)
			}
		}

		fmt.Printf("CA certificate: %s\n", result.CertPath)
		fmt.Printf("CA private key: %s\n", result.KeyPath)
	},
}

func init() {
	certificatesCAViper = viper.New()

	certificatesCmd.AddCommand(certificatesGenerateSelfSignedCACmd)

	certificatesGenerateSelfSignedCACmd.Flags().String("output-dir", "./certs", "Directory where generated certificates should be written")
	certificatesGenerateSelfSignedCACmd.Flags().String("ca-name", "infractl-dev-ca", "Logical name for the CA")
	certificatesGenerateSelfSignedCACmd.Flags().String("common-name", "Infractl Development CA", "CA certificate common name")
	certificatesGenerateSelfSignedCACmd.Flags().String("organization", "Infractl Development", "CA organization name")
	certificatesGenerateSelfSignedCACmd.Flags().StringSlice("san", nil, "Additional SAN entries to include in the CA certificate")
	certificatesGenerateSelfSignedCACmd.Flags().Int("years-valid", 10, "Number of years the generated CA should remain valid")

	certificatesCAViper.BindPFlag("cert_output_dir", certificatesGenerateSelfSignedCACmd.Flags().Lookup("output-dir"))
	certificatesCAViper.BindPFlag("ca_name", certificatesGenerateSelfSignedCACmd.Flags().Lookup("ca-name"))
	certificatesCAViper.BindPFlag("ca_common_name", certificatesGenerateSelfSignedCACmd.Flags().Lookup("common-name"))
	certificatesCAViper.BindPFlag("ca_organization", certificatesGenerateSelfSignedCACmd.Flags().Lookup("organization"))
	certificatesCAViper.BindPFlag("ca_sans", certificatesGenerateSelfSignedCACmd.Flags().Lookup("san"))
	certificatesCAViper.BindPFlag("cert_years_valid", certificatesGenerateSelfSignedCACmd.Flags().Lookup("years-valid"))
}

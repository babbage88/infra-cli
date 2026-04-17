package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	certrenew "github.com/babbage88/go-infra/webutils/cert_renew"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var certificatesCloudflareACMEViper *viper.Viper

var defaultRecursiveNameServers = []string{
	"1.1.1.1:53",
	"1.0.0.1:53",
	"8.8.8.8:53",
	"8.8.4.4:53",
}

var certificatesCloudflareACMEDNSCmd = &cobra.Command{
	Use:   "cloudflare-acme-dns",
	Short: "Renew an ACME certificate using the Cloudflare DNS challenge",
	Run: func(cmd *cobra.Command, args []string) {
		domainNames := certificatesCloudflareACMEViper.GetStringSlice("cert_domain_names")
		acmeEmail := certificatesCloudflareACMEViper.GetString("cert_acme_email")
		acmeURL := certificatesCloudflareACMEViper.GetString("cert_acme_url")
		zipDir := certificatesCloudflareACMEViper.GetString("cert_zip_dir")
		pushS3 := certificatesCloudflareACMEViper.GetBool("cert_push_s3")
		token := certificatesCloudflareACMEViper.GetString("cert_cf_token")
		recursiveNameServers := certificatesCloudflareACMEViper.GetStringSlice("cert_recursive_nameservers")
		timeoutSeconds := certificatesCloudflareACMEViper.GetInt("cert_timeout_seconds")
		outputDir := expandPath(certificatesCloudflareACMEViper.GetString("cert_output_dir"))
		writeFiles := certificatesCloudflareACMEViper.GetBool("cert_write_files")

		if len(domainNames) == 0 {
			slog.Error("At least one domain name is required", "hint", "pass one or more --domain flags")
			os.Exit(1)
		}
		if acmeEmail == "" {
			slog.Error("ACME email is required", "hint", "pass --acme-email or configure cert_acme_email")
			os.Exit(1)
		}
		if token == "" {
			slog.Error("Cloudflare token is required", "hint", "pass --cloudflare-token or configure cert_cf_token")
			os.Exit(1)
		}
		if timeoutSeconds <= 0 {
			slog.Error("Timeout must be greater than zero", "timeout_seconds", timeoutSeconds)
			os.Exit(1)
		}
		if len(recursiveNameServers) == 0 {
			recursiveNameServers = append([]string(nil), defaultRecursiveNameServers...)
		}

		req := certrenew.CertDnsRenewReq{
			DomainNames:          domainNames,
			AcmeEmail:            acmeEmail,
			AcmeUrl:              acmeURL,
			ZipDir:               zipDir,
			PushS3:               pushS3,
			Token:                token,
			RecursiveNameServers: recursiveNameServers,
			Timeout:              time.Duration(timeoutSeconds) * time.Second,
		}

		slog.Info("Renewing ACME certificate via Cloudflare DNS", "domains", domainNames, "acme_url", acmeURL, "recursive_nameservers", recursiveNameServers)

		certData, err := req.Renew()
		if err != nil {
			slog.Error("Failed renewing ACME certificate", "error", err.Error())
			os.Exit(1)
		}

		if writeFiles {
			writtenFiles, writeErr := writeRenewedCertificateFiles(outputDir, domainNames[0], certData)
			if writeErr != nil {
				slog.Error("Failed writing renewed certificate files", "error", writeErr.Error())
				os.Exit(1)
			}
			for label, path := range writtenFiles {
				fmt.Printf("%s: %s\n", label, path)
			}
		}

		fmt.Printf("Domains: %v\n", certData.DomainNames)
		fmt.Printf("Zip path: %s\n", certData.ZipDir)
		if certData.S3DownloadUrl != "" {
			fmt.Printf("S3 download URL: %s\n", certData.S3DownloadUrl)
		}
	},
}

func init() {
	certificatesCloudflareACMEViper = viper.New()

	certificatesCmd.AddCommand(certificatesCloudflareACMEDNSCmd)

	certificatesCloudflareACMEDNSCmd.Flags().StringSlice("domain", nil, "Domain names to include in the certificate; repeat flag for multiple SANs")
	certificatesCloudflareACMEDNSCmd.Flags().String("acme-email", "", "ACME account email address")
	certificatesCloudflareACMEDNSCmd.Flags().String("acme-url", "https://acme-v02.api.letsencrypt.org/directory", "ACME directory URL")
	certificatesCloudflareACMEDNSCmd.Flags().String("zip-dir", "", "Optional zip filename prefix/directory name used by the renewal package")
	certificatesCloudflareACMEDNSCmd.Flags().Bool("push-s3", false, "Push the resulting certificate bundle to S3")
	certificatesCloudflareACMEDNSCmd.Flags().String("cloudflare-token", "", "Cloudflare API token for DNS challenge validation")
	certificatesCloudflareACMEDNSCmd.Flags().StringSlice("recursive-nameserver", defaultRecursiveNameServers, "Recursive DNS servers to use for propagation checks")
	certificatesCloudflareACMEDNSCmd.Flags().Int("timeout-seconds", 120, "DNS challenge timeout in seconds")
	certificatesCloudflareACMEDNSCmd.Flags().String("output-dir", "./certs", "Directory to write renewed certificate files when --write-files is enabled")
	certificatesCloudflareACMEDNSCmd.Flags().Bool("write-files", true, "Write the renewed certificate, chain, fullchain, and private key to local files")

	certificatesCloudflareACMEViper.BindPFlag("cert_domain_names", certificatesCloudflareACMEDNSCmd.Flags().Lookup("domain"))
	certificatesCloudflareACMEViper.BindPFlag("cert_acme_email", certificatesCloudflareACMEDNSCmd.Flags().Lookup("acme-email"))
	certificatesCloudflareACMEViper.BindPFlag("cert_acme_url", certificatesCloudflareACMEDNSCmd.Flags().Lookup("acme-url"))
	certificatesCloudflareACMEViper.BindPFlag("cert_zip_dir", certificatesCloudflareACMEDNSCmd.Flags().Lookup("zip-dir"))
	certificatesCloudflareACMEViper.BindPFlag("cert_push_s3", certificatesCloudflareACMEDNSCmd.Flags().Lookup("push-s3"))
	certificatesCloudflareACMEViper.BindPFlag("cert_cf_token", certificatesCloudflareACMEDNSCmd.Flags().Lookup("cloudflare-token"))
	certificatesCloudflareACMEViper.BindPFlag("cert_recursive_nameservers", certificatesCloudflareACMEDNSCmd.Flags().Lookup("recursive-nameserver"))
	certificatesCloudflareACMEViper.BindPFlag("cert_timeout_seconds", certificatesCloudflareACMEDNSCmd.Flags().Lookup("timeout-seconds"))
	certificatesCloudflareACMEViper.BindPFlag("cert_output_dir", certificatesCloudflareACMEDNSCmd.Flags().Lookup("output-dir"))
	certificatesCloudflareACMEViper.BindPFlag("cert_write_files", certificatesCloudflareACMEDNSCmd.Flags().Lookup("write-files"))
}

func writeRenewedCertificateFiles(outputDir, primaryDomain string, certData *certrenew.CertificateData) (map[string]string, error) {
	if outputDir == "" {
		outputDir = "./certs"
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}

	baseName := sanitizeFilename(primaryDomain)
	files := map[string]string{
		"Certificate": filepath.Join(outputDir, baseName+".crt"),
		"Chain":       filepath.Join(outputDir, baseName+".chain.crt"),
		"Fullchain":   filepath.Join(outputDir, baseName+".fullchain.crt"),
		"Private key": filepath.Join(outputDir, baseName+".key"),
	}

	writePairs := map[string]string{
		files["Certificate"]: certData.CertPEM,
		files["Chain"]:       certData.ChainPEM,
		files["Fullchain"]:   certData.Fullchain,
		files["Private key"]: certData.PrivKey,
	}

	for path, content := range writePairs {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return nil, fmt.Errorf("write certificate file %q: %w", path, err)
		}
	}

	return files, nil
}

func sanitizeFilename(value string) string {
	replacer := []string{"*", "wildcard", "/", "_", "\\", "_", ":", "_", " ", "_"}
	return strings.NewReplacer(replacer...).Replace(value)
}

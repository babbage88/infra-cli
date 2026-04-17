package deployer

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/babbage88/goph/v2"
	"github.com/babbage88/infra-cli/ssh"
)

type GenerateCARequest struct {
	OutputDir    string
	CAName       string
	CommonName   string
	Organization string
	SANs         []string
	YearsValid   int
}

type GenerateAppCertRequest struct {
	OutputDir    string
	CAName       string
	AppName      string
	CommonName   string
	Organization string
	SANs         []string
	YearsValid   int
}

type CertificateFiles struct {
	CertPath   string
	KeyPath    string
	CACertPath string
}

type RemoteTrustCARequest struct {
	Hostname   string
	Username   string
	SSHKey     string
	Passphrase string
	UseAgent   bool
	Port       uint
	CertPath   string
	CAName     string
}

func GenerateAndTrustLocalCA(req GenerateCARequest) (*CertificateFiles, error) {
	if req.OutputDir == "" {
		req.OutputDir = "./certs"
	}
	if req.CAName == "" {
		req.CAName = "infractl-dev-ca"
	}
	if req.CommonName == "" {
		req.CommonName = "Infractl Development CA"
	}
	if req.Organization == "" {
		req.Organization = "Infractl Development"
	}
	if req.YearsValid <= 0 {
		req.YearsValid = 10
	}

	certPath := filepath.Join(req.OutputDir, req.CAName+".crt")
	keyPath := filepath.Join(req.OutputDir, req.CAName+".key")

	if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}

	priv, certDER, err := generateCACert(req.CommonName, req.Organization, req.SANs, req.YearsValid)
	if err != nil {
		return nil, err
	}

	if err := writeCertAndKey(certPath, keyPath, certDER, priv); err != nil {
		return nil, err
	}

	if err := trustLocalCertificate(certPath, req.CAName); err != nil {
		return nil, err
	}

	return &CertificateFiles{
		CertPath: certPath,
		KeyPath:  keyPath,
	}, nil
}

func GenerateAppCertificate(req GenerateAppCertRequest) (*CertificateFiles, error) {
	if req.OutputDir == "" {
		req.OutputDir = "./certs"
	}
	if req.CAName == "" {
		req.CAName = "infractl-dev-ca"
	}
	if req.AppName == "" {
		return nil, fmt.Errorf("application certificate name is required")
	}
	if req.CommonName == "" {
		req.CommonName = "localhost"
	}
	if req.Organization == "" {
		req.Organization = "Infractl Development"
	}
	if req.YearsValid <= 0 {
		req.YearsValid = 2
	}

	caCertPath := filepath.Join(req.OutputDir, req.CAName+".crt")
	caKeyPath := filepath.Join(req.OutputDir, req.CAName+".key")
	appCertPath := filepath.Join(req.OutputDir, req.AppName+".crt")
	appKeyPath := filepath.Join(req.OutputDir, req.AppName+".key")

	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("read CA certificate %q: %w", caCertPath, err)
	}
	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read CA private key %q: %w", caKeyPath, err)
	}

	caCert, err := parseCertificatePEM(caCertPEM)
	if err != nil {
		return nil, fmt.Errorf("parse CA certificate: %w", err)
	}
	caKey, err := parseECPrivateKeyPEM(caKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse CA private key: %w", err)
	}

	appKey, certDER, err := generateSignedCertificate(caCert, caKey, req.CommonName, req.Organization, req.SANs, req.YearsValid)
	if err != nil {
		return nil, err
	}

	if err := writeCertAndKey(appCertPath, appKeyPath, certDER, appKey); err != nil {
		return nil, err
	}

	return &CertificateFiles{
		CertPath:   appCertPath,
		KeyPath:    appKeyPath,
		CACertPath: caCertPath,
	}, nil
}

func TrustCARemoteViaSSH(req RemoteTrustCARequest) error {
	client, err := ssh.InitializeSshClient(req.Hostname, req.Username, req.SSHKey, req.Passphrase, req.UseAgent, req.Port)
	if err != nil {
		return err
	}
	defer client.Close()

	certBytes, err := os.ReadFile(req.CertPath)
	if err != nil {
		return fmt.Errorf("read CA certificate %q: %w", req.CertPath, err)
	}

	if out, err := client.Run("uname -s"); err == nil {
		osName := strings.TrimSpace(string(out))
		if osName == "Darwin" || osName == "Linux" {
			return trustCAPOSIXRemote(client, req.CAName, certBytes, osName)
		}
	}

	return trustCAWindowsRemote(client, req.CAName, certBytes)
}

func trustCAPOSIXRemote(client *goph.Client, caName string, certBytes []byte, osName string) error {
	escapedCert := strings.ReplaceAll(string(certBytes), "\r\n", "\n")
	escapedCert = strings.ReplaceAll(escapedCert, "\n", "\\n")

	installScript := fmt.Sprintf(`set -e
if [ %s = "Darwin" ]; then
  cert_path="/tmp/%s.crt"
  printf '%%b' %s > "$cert_path"
  sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "$cert_path"
  exit 0
fi
if [ %s = "Linux" ]; then
  cert_path="/tmp/%s.crt"
  printf '%%b' %s > "$cert_path"
  if [ -d /usr/local/share/ca-certificates ]; then
    sudo install -m 0644 "$cert_path" /usr/local/share/ca-certificates/%s.crt
    sudo update-ca-certificates
    exit 0
  fi
  if [ -d /etc/pki/ca-trust/source/anchors ]; then
    sudo install -m 0644 "$cert_path" /etc/pki/ca-trust/source/anchors/%s.crt
    sudo update-ca-trust extract
    exit 0
  fi
  echo "unsupported Linux trust store layout" >&2
  exit 1
fi
echo "unsupported remote POSIX OS for trust installation" >&2
exit 1`,
		shellQuote(osName),
		caName,
		shellQuote(escapedCert),
		shellQuote(osName),
		caName,
		shellQuote(escapedCert),
		caName,
		caName,
	)

	out, err := client.Run("sh -c " + shellQuote(installScript))
	if err != nil {
		return formatRemoteCommandError(fmt.Errorf("trust CA on remote host: %w", err), out)
	}
	return nil
}

func trustCAWindowsRemote(client *goph.Client, caName string, certBytes []byte) error {
	psScript := fmt.Sprintf(`$certPath = Join-Path $env:TEMP '%s.crt'
[System.IO.File]::WriteAllBytes($certPath, [System.Convert]::FromBase64String('%s'))
Import-Certificate -FilePath $certPath -CertStoreLocation Cert:\LocalMachine\Root | Out-Null`,
		caName,
		base64.StdEncoding.EncodeToString(certBytes),
	)
	out, err := client.Run("powershell -NoProfile -NonInteractive -Command " + shellQuote(psScript))
	if err != nil {
		return formatRemoteCommandError(fmt.Errorf("trust CA on remote Windows host: %w", err), out)
	}
	return nil
}

func generateCACert(commonName, organization string, sans []string, yearsValid int) (*ecdsa.PrivateKey, []byte, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate CA private key: %w", err)
	}

	serial, err := randSerialNumber()
	if err != nil {
		return nil, nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{organization},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(yearsValid, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}
	addSANs(tmpl, sans)

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("create CA certificate: %w", err)
	}

	return priv, der, nil
}

func generateSignedCertificate(caCert *x509.Certificate, caKey *ecdsa.PrivateKey, commonName, organization string, sans []string, yearsValid int) (*ecdsa.PrivateKey, []byte, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate application private key: %w", err)
	}

	serial, err := randSerialNumber()
	if err != nil {
		return nil, nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{organization},
		},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    time.Now().AddDate(yearsValid, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	addSANs(tmpl, sans)
	if len(tmpl.DNSNames) == 0 && len(tmpl.IPAddresses) == 0 {
		tmpl.DNSNames = []string{commonName}
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &priv.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("create application certificate: %w", err)
	}

	return priv, der, nil
}

func writeCertAndKey(certPath, keyPath string, certDER []byte, key *ecdsa.PrivateKey) error {
	certOut, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("create certificate file %q: %w", certPath, err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		certOut.Close()
		return fmt.Errorf("write certificate file %q: %w", certPath, err)
	}
	if err := certOut.Close(); err != nil {
		return fmt.Errorf("close certificate file %q: %w", certPath, err)
	}

	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}
	keyOut, err := os.OpenFile(keyPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create private key file %q: %w", keyPath, err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		keyOut.Close()
		return fmt.Errorf("write private key file %q: %w", keyPath, err)
	}
	if err := keyOut.Close(); err != nil {
		return fmt.Errorf("close private key file %q: %w", keyPath, err)
	}

	return nil
}

func trustLocalCertificate(certPath, caName string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("sudo", "security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", "/Library/Keychains/System.keychain", certPath)
	case "linux":
		script := fmt.Sprintf(`set -e
if [ -d /usr/local/share/ca-certificates ]; then
  sudo install -m 0644 %s /usr/local/share/ca-certificates/%s.crt
  sudo update-ca-certificates
  exit 0
fi
if [ -d /etc/pki/ca-trust/source/anchors ]; then
  sudo install -m 0644 %s /etc/pki/ca-trust/source/anchors/%s.crt
  sudo update-ca-trust extract
  exit 0
fi
echo "unsupported Linux trust store layout" >&2
exit 1`, shellQuote(certPath), caName, shellQuote(certPath), caName)
		cmd = exec.Command("sh", "-c", script)
	case "windows":
		cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", fmt.Sprintf("Import-Certificate -FilePath '%s' -CertStoreLocation Cert:\\LocalMachine\\Root | Out-Null", strings.ReplaceAll(certPath, `'`, `''`)))
	default:
		return fmt.Errorf("unsupported local OS for trust installation: %s", runtime.GOOS)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("trust local CA certificate: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func parseCertificatePEM(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM certificate")
	}
	return x509.ParseCertificate(block.Bytes)
}

func parseECPrivateKeyPEM(data []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM private key")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

func randSerialNumber() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("generate certificate serial number: %w", err)
	}
	return serial, nil
}

func addSANs(tmpl *x509.Certificate, sans []string) {
	for _, san := range sans {
		san = strings.TrimSpace(san)
		if san == "" {
			continue
		}
		if ip := net.ParseIP(san); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, san)
		}
	}
}

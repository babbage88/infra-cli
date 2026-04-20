package infractl_services

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/babbage88/infra-cli/deployer"
	coredeploy "github.com/babbage88/infra-core/deployment"
)

func BoolPtr(value bool) *bool {
	return &value
}

func DefaultGarageTokenRequest() coredeploy.GarageTokenRequest {
	return coredeploy.GarageTokenRequest{
		CreateBucket:       BoolPtr(true),
		AllowCreateBuckets: BoolPtr(false),
		AllowRead:          BoolPtr(true),
		AllowWrite:         BoolPtr(true),
		AllowOwner:         BoolPtr(true),
		BinaryPath:         "/usr/local/bin/garage",
		ConfigPath:         "/etc/garage.toml",
		LayoutZone:         "dc1",
		LayoutCapacity:     "100G",
	}
}

func MergeGarageTokenDefaults(req coredeploy.GarageTokenRequest, defaults coredeploy.GarageTokenRequest) coredeploy.GarageTokenRequest {
	if strings.TrimSpace(req.BucketName) == "" {
		req.BucketName = defaults.BucketName
	}
	if strings.TrimSpace(req.KeyName) == "" {
		req.KeyName = defaults.KeyName
	}
	if req.CreateBucket == nil {
		req.CreateBucket = defaults.CreateBucket
	}
	if req.AllowCreateBuckets == nil {
		req.AllowCreateBuckets = defaults.AllowCreateBuckets
	}
	if boolValue(req.AllowCreateBuckets) && strings.TrimSpace(req.BucketName) == "" && req.AllowRead == nil && req.AllowWrite == nil && req.AllowOwner == nil {
		req.AllowRead = BoolPtr(false)
		req.AllowWrite = BoolPtr(false)
		req.AllowOwner = BoolPtr(false)
	}
	if req.AllowRead == nil {
		req.AllowRead = defaults.AllowRead
	}
	if req.AllowWrite == nil {
		req.AllowWrite = defaults.AllowWrite
	}
	if req.AllowOwner == nil {
		req.AllowOwner = defaults.AllowOwner
	}
	if strings.TrimSpace(req.BinaryPath) == "" {
		req.BinaryPath = defaults.BinaryPath
	}
	if strings.TrimSpace(req.ConfigPath) == "" {
		req.ConfigPath = defaults.ConfigPath
	}
	if strings.TrimSpace(req.S3Endpoint) == "" {
		req.S3Endpoint = defaults.S3Endpoint
	}
	if strings.TrimSpace(req.LayoutZone) == "" {
		req.LayoutZone = defaults.LayoutZone
	}
	if strings.TrimSpace(req.LayoutCapacity) == "" {
		req.LayoutCapacity = defaults.LayoutCapacity
	}
	req.SSH = MergeSSHDefaults(req.SSH, defaults.SSH)
	return req
}

func CreateGarageToken(req coredeploy.GarageTokenRequest) (coredeploy.GarageTokenResult, error) {
	if strings.TrimSpace(req.SSH.Host) == "" {
		return coredeploy.GarageTokenResult{}, fmt.Errorf("ssh.host is required")
	}
	if strings.TrimSpace(req.SSH.User) == "" {
		return coredeploy.GarageTokenResult{}, fmt.Errorf("ssh.user is required")
	}
	if strings.TrimSpace(req.KeyName) == "" {
		return coredeploy.GarageTokenResult{}, fmt.Errorf("key_name is required")
	}

	createBucket := boolValue(req.CreateBucket)
	allowCreateBuckets := boolValue(req.AllowCreateBuckets)
	allowRead := boolValue(req.AllowRead)
	allowWrite := boolValue(req.AllowWrite)
	allowOwner := boolValue(req.AllowOwner)

	if strings.TrimSpace(req.BucketName) == "" && !allowCreateBuckets {
		return coredeploy.GarageTokenResult{}, fmt.Errorf("bucket_name is required unless allow_create_buckets is true")
	}
	if strings.TrimSpace(req.S3Endpoint) == "" {
		req.S3Endpoint = fmt.Sprintf("http://%s:3900", req.SSH.Host)
	}

	installer, err := deployer.NewRemoteGarageInstallerWithSsh(
		req.SSH.Host,
		req.SSH.User,
		req.SSH.KeyPath,
		req.SSH.Passphrase,
		req.SSH.UseAgent,
		req.SSH.Port,
	)
	if err != nil {
		return coredeploy.GarageTokenResult{}, fmt.Errorf("initialize SSH client: %w", err)
	}
	defer installer.SshClient.Close()

	tokenReq := deployer.GarageTokenRequest{
		BucketName:         req.BucketName,
		KeyName:            req.KeyName,
		CreateBucket:       createBucket,
		AllowCreateBuckets: allowCreateBuckets,
		AllowRead:          allowRead,
		AllowWrite:         allowWrite,
		AllowOwner:         allowOwner,
		BinaryPath:         req.BinaryPath,
		ConfigPath:         req.ConfigPath,
		LayoutZone:         req.LayoutZone,
		LayoutCapacity:     req.LayoutCapacity,
	}
	creds, err := installer.CreateS3Token(tokenReq)
	if err != nil {
		return coredeploy.GarageTokenResult{}, err
	}

	return coredeploy.GarageTokenResult{
		Host:              req.SSH.Host,
		S3Endpoint:        req.S3Endpoint,
		BucketName:        creds.BucketName,
		KeyName:           creds.KeyName,
		AccessKeyID:       creds.AccessKeyID,
		SecretAccessKey:   creds.SecretAccessKey,
		MCAliasSetCommand: fmt.Sprintf("mc alias set garage %s %s %s", req.S3Endpoint, creds.AccessKeyID, creds.SecretAccessKey),
	}, nil
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func DefaultGarageNodeRequest() coredeploy.GarageNodeRequest {
	return coredeploy.GarageNodeRequest{
		Version:           "v2.2.0",
		BinaryPath:        "/usr/local/bin/garage",
		ConfigPath:        "/etc/garage.toml",
		MetadataDir:       "/var/lib/garage/meta",
		DataDir:           "/var/lib/garage/data",
		DBEngine:          "sqlite",
		ReplicationFactor: 1,
		RPCBindAddr:       "[::]:3901",
		S3APIBindAddr:     "[::]:3900",
		S3Region:          "garage",
		S3RootDomain:      ".s3.local",
		S3WebBindAddr:     "[::]:3902",
		S3WebRootDomain:   ".web.local",
		S3WebIndex:        "index.html",
		K2VAPIBindAddr:    "[::]:3904",
		AdminAPIBindAddr:  "[::]:3903",
		LogLevel:          "garage=info",
	}
}

func MergeGarageNodeDefaults(req coredeploy.GarageNodeRequest, defaults coredeploy.GarageNodeRequest) coredeploy.GarageNodeRequest {
	if strings.TrimSpace(req.Version) == "" {
		req.Version = defaults.Version
	}
	if strings.TrimSpace(req.BinaryPath) == "" {
		req.BinaryPath = defaults.BinaryPath
	}
	if strings.TrimSpace(req.ConfigPath) == "" {
		req.ConfigPath = defaults.ConfigPath
	}
	if strings.TrimSpace(req.MetadataDir) == "" {
		req.MetadataDir = defaults.MetadataDir
	}
	if strings.TrimSpace(req.DataDir) == "" {
		req.DataDir = defaults.DataDir
	}
	if strings.TrimSpace(req.DBEngine) == "" {
		req.DBEngine = defaults.DBEngine
	}
	if req.ReplicationFactor == 0 {
		req.ReplicationFactor = defaults.ReplicationFactor
	}
	if strings.TrimSpace(req.RPCBindAddr) == "" {
		req.RPCBindAddr = defaults.RPCBindAddr
	}
	if strings.TrimSpace(req.RPCPublicAddr) == "" {
		req.RPCPublicAddr = defaults.RPCPublicAddr
	}
	if strings.TrimSpace(req.RPCSecret) == "" {
		req.RPCSecret = defaults.RPCSecret
	}
	if strings.TrimSpace(req.S3APIBindAddr) == "" {
		req.S3APIBindAddr = defaults.S3APIBindAddr
	}
	if strings.TrimSpace(req.S3Region) == "" {
		req.S3Region = defaults.S3Region
	}
	if strings.TrimSpace(req.S3RootDomain) == "" {
		req.S3RootDomain = defaults.S3RootDomain
	}
	if strings.TrimSpace(req.S3WebBindAddr) == "" {
		req.S3WebBindAddr = defaults.S3WebBindAddr
	}
	if strings.TrimSpace(req.S3WebRootDomain) == "" {
		req.S3WebRootDomain = defaults.S3WebRootDomain
	}
	if strings.TrimSpace(req.S3WebIndex) == "" {
		req.S3WebIndex = defaults.S3WebIndex
	}
	if strings.TrimSpace(req.K2VAPIBindAddr) == "" {
		req.K2VAPIBindAddr = defaults.K2VAPIBindAddr
	}
	if strings.TrimSpace(req.AdminAPIBindAddr) == "" {
		req.AdminAPIBindAddr = defaults.AdminAPIBindAddr
	}
	if strings.TrimSpace(req.AdminToken) == "" {
		req.AdminToken = defaults.AdminToken
	}
	if strings.TrimSpace(req.MetricsToken) == "" {
		req.MetricsToken = defaults.MetricsToken
	}
	if strings.TrimSpace(req.LogLevel) == "" {
		req.LogLevel = defaults.LogLevel
	}
	req.SSH = MergeSSHDefaults(req.SSH, defaults.SSH)
	return req
}

func DeployGarageNode(req coredeploy.GarageNodeRequest) (coredeploy.GarageNodeResult, error) {
	if strings.TrimSpace(req.SSH.Host) == "" {
		return coredeploy.GarageNodeResult{}, fmt.Errorf("ssh.host is required")
	}
	if strings.TrimSpace(req.SSH.User) == "" {
		return coredeploy.GarageNodeResult{}, fmt.Errorf("ssh.user is required")
	}
	if req.ReplicationFactor <= 0 {
		return coredeploy.GarageNodeResult{}, fmt.Errorf("replication_factor must be greater than zero")
	}
	if strings.TrimSpace(req.RPCPublicAddr) == "" {
		req.RPCPublicAddr = fmt.Sprintf("%s:3901", req.SSH.Host)
	}

	var err error
	if strings.TrimSpace(req.RPCSecret) == "" {
		req.RPCSecret, err = randomHexString(32)
		if err != nil {
			return coredeploy.GarageNodeResult{}, err
		}
	}
	if strings.TrimSpace(req.AdminToken) == "" {
		req.AdminToken, err = randomBase64String(32)
		if err != nil {
			return coredeploy.GarageNodeResult{}, err
		}
	}
	if strings.TrimSpace(req.MetricsToken) == "" {
		req.MetricsToken, err = randomBase64String(32)
		if err != nil {
			return coredeploy.GarageNodeResult{}, err
		}
	}

	installer, err := deployer.NewRemoteGarageInstallerWithSsh(
		req.SSH.Host,
		req.SSH.User,
		req.SSH.KeyPath,
		req.SSH.Passphrase,
		req.SSH.UseAgent,
		req.SSH.Port,
	)
	if err != nil {
		return coredeploy.GarageNodeResult{}, fmt.Errorf("initialize SSH client: %w", err)
	}
	defer installer.SshClient.Close()

	cfg := deployer.GarageNodeConfig{
		Version:           req.Version,
		BinaryPath:        req.BinaryPath,
		ConfigPath:        req.ConfigPath,
		MetadataDir:       req.MetadataDir,
		DataDir:           req.DataDir,
		DBEngine:          req.DBEngine,
		ReplicationFactor: req.ReplicationFactor,
		RPCBindAddr:       req.RPCBindAddr,
		RPCPublicAddr:     req.RPCPublicAddr,
		RPCSecret:         req.RPCSecret,
		S3Region:          req.S3Region,
		S3APIBindAddr:     req.S3APIBindAddr,
		S3RootDomain:      req.S3RootDomain,
		S3WebBindAddr:     req.S3WebBindAddr,
		S3WebRootDomain:   req.S3WebRootDomain,
		S3WebIndex:        req.S3WebIndex,
		K2VAPIBindAddr:    req.K2VAPIBindAddr,
		AdminAPIBindAddr:  req.AdminAPIBindAddr,
		AdminToken:        req.AdminToken,
		MetricsToken:      req.MetricsToken,
		LogLevel:          req.LogLevel,
	}
	if err := installer.EnsureInstalledAndConfigured(cfg); err != nil {
		return coredeploy.GarageNodeResult{}, err
	}

	return coredeploy.GarageNodeResult{
		Host:          req.SSH.Host,
		BinaryPath:    req.BinaryPath,
		ConfigPath:    req.ConfigPath,
		ServiceName:   "garage",
		RPCPublicAddr: req.RPCPublicAddr,
		S3Endpoint:    fmt.Sprintf("http://%s", garageS3BindAddrToAdvertised(req.SSH.Host, req.S3APIBindAddr)),
		AdminEndpoint: fmt.Sprintf("http://%s", garageBindAddrToAdvertised(req.SSH.Host, req.AdminAPIBindAddr)),
		AdminToken:    req.AdminToken,
		MetricsToken:  req.MetricsToken,
	}, nil
}

func randomHexString(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random hex string: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func randomBase64String(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random base64 string: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

func garageBindAddrToAdvertised(host, bindAddr string) string {
	switch bindAddr {
	case "", "[::]:3903", "0.0.0.0:3903":
		return fmt.Sprintf("%s:3903", host)
	case "[::]:3900", "0.0.0.0:3900":
		return fmt.Sprintf("%s:3900", host)
	case "[::]:3901", "0.0.0.0:3901":
		return fmt.Sprintf("%s:3901", host)
	case "[::]:3902", "0.0.0.0:3902":
		return fmt.Sprintf("%s:3902", host)
	case "[::]:3904", "0.0.0.0:3904":
		return fmt.Sprintf("%s:3904", host)
	default:
		return bindAddr
	}
}

func garageS3BindAddrToAdvertised(host, bindAddr string) string {
	switch bindAddr {
	case "", "[::]:3900", "0.0.0.0:3900":
		return fmt.Sprintf("%s:3900", host)
	default:
		return bindAddr
	}
}

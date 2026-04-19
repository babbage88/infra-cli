package infractl_services

import (
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

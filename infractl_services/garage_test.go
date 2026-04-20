package infractl_services

import (
	"testing"

	coredeploy "github.com/babbage88/infra-core/deployment"
)

func TestMergeGarageTokenDefaultsPreservesExplicitFalsePermissions(t *testing.T) {
	req := coredeploy.GarageTokenRequest{
		KeyName:    "app-key",
		BucketName: "app-bucket",
		AllowRead:  BoolPtr(false),
	}

	merged := MergeGarageTokenDefaults(req, DefaultGarageTokenRequest())
	if merged.AllowRead == nil {
		t.Fatal("expected allow_read to be set")
	}
	if *merged.AllowRead {
		t.Fatal("expected explicit allow_read=false to be preserved")
	}
	if merged.AllowWrite == nil || !*merged.AllowWrite {
		t.Fatal("expected omitted allow_write to use default true")
	}
}

func TestMergeGarageTokenDefaultsAllowsClusterWideBucketCreationWithoutBucketPermissions(t *testing.T) {
	req := coredeploy.GarageTokenRequest{
		KeyName:            "cluster-key",
		AllowCreateBuckets: BoolPtr(true),
	}

	merged := MergeGarageTokenDefaults(req, DefaultGarageTokenRequest())
	if merged.AllowRead == nil || *merged.AllowRead {
		t.Fatal("expected allow_read=false when allow_create_buckets is true without a bucket")
	}
	if merged.AllowWrite == nil || *merged.AllowWrite {
		t.Fatal("expected allow_write=false when allow_create_buckets is true without a bucket")
	}
	if merged.AllowOwner == nil || *merged.AllowOwner {
		t.Fatal("expected allow_owner=false when allow_create_buckets is true without a bucket")
	}
}

func TestMergeGarageNodeDefaultsFillsExpectedValues(t *testing.T) {
	req := coredeploy.GarageNodeRequest{
		ReplicationFactor: 3,
		S3Region:          "us-test-1",
	}

	merged := MergeGarageNodeDefaults(req, DefaultGarageNodeRequest())
	if merged.ReplicationFactor != 3 {
		t.Fatalf("expected explicit replication factor to be preserved, got %d", merged.ReplicationFactor)
	}
	if merged.S3Region != "us-test-1" {
		t.Fatalf("expected explicit S3 region to be preserved, got %q", merged.S3Region)
	}
	if merged.Version == "" {
		t.Fatal("expected default Garage version")
	}
	if merged.BinaryPath != "/usr/local/bin/garage" {
		t.Fatalf("expected default binary path, got %q", merged.BinaryPath)
	}
}

func TestGarageBindAddrToAdvertisedUsesHostForWildcardBinds(t *testing.T) {
	if got := garageS3BindAddrToAdvertised("garage-01", "[::]:3900"); got != "garage-01:3900" {
		t.Fatalf("expected host-based S3 endpoint, got %q", got)
	}
	if got := garageBindAddrToAdvertised("garage-01", "[::]:3903"); got != "garage-01:3903" {
		t.Fatalf("expected host-based admin endpoint, got %q", got)
	}
}

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

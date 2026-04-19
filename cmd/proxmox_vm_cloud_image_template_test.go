package cmd

import (
	"strings"
	"testing"
)

func TestParseCloudImageImportVolID(t *testing.T) {
	output := `download complete
IMPORTED_VOLID=local-lvm:vm-9001-disk-0
`
	got := parseCloudImageImportVolID(output)
	if got != "local-lvm:vm-9001-disk-0" {
		t.Fatalf("unexpected imported volume ID: %q", got)
	}
}

func TestRenderVMCloudImageImportScriptKeepsURLQuoted(t *testing.T) {
	req := &vmCloudImageTemplateRequest{
		VMID:         9001,
		ImageURL:     "https://example.com/images/noble cloud.img",
		Storage:      "local-lvm",
		CleanupImage: true,
	}

	script := renderVMCloudImageImportScript(req)
	for _, want := range []string{
		"qm importdisk",
		"IMPORTED_VOLID=$imported",
		"curl -fL --retry 3",
		"wget -O",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q\n%s", want, script)
		}
	}
	if strings.Contains(script, "image_url=https://example.com/images/noble cloud.img\n") {
		t.Fatalf("script includes unquoted image URL assignment:\n%s", script)
	}
}

func TestValidateVMCloudImageTemplateRequestDefaults(t *testing.T) {
	req := &vmCloudImageTemplateRequest{
		Node:     "pve01",
		VMID:     9001,
		Name:     "ubuntu-template",
		ImageURL: "https://example.com/noble-server-cloudimg-amd64.img",
		Storage:  "local-lvm",
	}

	if err := validateVMCloudImageTemplateRequest(req); err != nil {
		t.Fatalf("validate request: %v", err)
	}
	if req.DiskBus != "scsi0" {
		t.Fatalf("expected default disk bus scsi0, got %q", req.DiskBus)
	}
	if req.BootOrder != "scsi0" {
		t.Fatalf("expected boot order to default to disk bus, got %q", req.BootOrder)
	}
	if req.CloudInitStorage != "local-lvm" {
		t.Fatalf("expected cloud-init storage to default to storage, got %q", req.CloudInitStorage)
	}
}

func TestCloudImageFilenameIgnoresQueryString(t *testing.T) {
	got := cloudImageFilename("https://example.com/images/debian-12.qcow2?download=1", 9001)
	if got != "debian-12.qcow2" {
		t.Fatalf("unexpected filename: %q", got)
	}
}

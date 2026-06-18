package cmd

import "testing"

func TestProxmoxNewAPITokenWriteDefaultConfigFlagDefaultsToFalse(t *testing.T) {
	flag := proxmoxNewAPITokenCmd.Flags().Lookup("write-default-config")
	if flag == nil {
		t.Fatal("write-default-config flag not found")
	}
	if flag.DefValue != "false" {
		t.Fatalf("write-default-config default = %q, want false", flag.DefValue)
	}
}

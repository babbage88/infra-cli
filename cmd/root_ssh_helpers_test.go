package cmd

import (
	"testing"

	"github.com/spf13/viper"
)

func TestResolveRootSSHOptionsPromptsForHostWhenInteractive(t *testing.T) {
	previousRootViper := rootViperCfg
	previousPrompt := promptForRootSSHHost
	previousInteractiveCheck := tuiIsInteractive
	t.Cleanup(func() {
		rootViperCfg = previousRootViper
		promptForRootSSHHost = previousPrompt
		tuiIsInteractive = previousInteractiveCheck
	})

	rootViperCfg = viper.New()
	rootViperCfg.Set("ssh_remote_user", "deploy")
	rootViperCfg.Set("ssh_port", 22)

	tuiIsInteractive = func() bool { return true }
	promptForRootSSHHost = func(defaultHost string) string {
		if defaultHost != "" {
			t.Fatalf("expected empty default host, got %q", defaultHost)
		}
		return "db01.example.com"
	}

	opts, err := resolveRootSSHOptions("", "")
	if err != nil {
		t.Fatalf("resolveRootSSHOptions returned error: %v", err)
	}
	if opts.Host != "db01.example.com" {
		t.Fatalf("expected prompted host, got %q", opts.Host)
	}
	if opts.User != "deploy" {
		t.Fatalf("expected configured user, got %q", opts.User)
	}
}

func TestResolveRootSSHOptionsErrorsWhenHostMissingAndNonInteractive(t *testing.T) {
	previousRootViper := rootViperCfg
	previousInteractiveCheck := tuiIsInteractive
	t.Cleanup(func() {
		rootViperCfg = previousRootViper
		tuiIsInteractive = previousInteractiveCheck
	})

	rootViperCfg = viper.New()
	rootViperCfg.Set("ssh_port", 22)
	tuiIsInteractive = func() bool { return false }

	if _, err := resolveRootSSHOptions("", ""); err == nil {
		t.Fatal("expected missing host error, got nil")
	}
}

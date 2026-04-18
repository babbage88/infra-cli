package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/babbage88/goph/v2"
	infraSSH "github.com/babbage88/infra-cli/ssh"
)

type proxmoxNewUserOptions struct {
	PveNode  string
	Username string
	Realm    string
	Comment  string
	Password string
	Force    bool
}

type proxmoxNewTokenOptions struct {
	PveNode            string
	UserID             string
	Username           string
	Realm              string
	TokenID            string
	Comment            string
	Role               string
	ACLPath            string
	Privsep            bool
	WriteDefaultConfig bool
	Force              bool
}

type proxmoxCreatedToken struct {
	FullTokenID string
	Secret      string
}

var (
	proxmoxNewUserFlags  proxmoxNewUserOptions
	proxmoxNewTokenFlags proxmoxNewTokenOptions
)

func initializeProxmoxAdminSSH(pveNode string) (*goph.Client, error) {
	sshHost := strings.TrimSpace(rootViperCfg.GetString("ssh_remote_host"))
	if sshHost == "" {
		sshHost = strings.TrimSpace(pveNode)
	}
	if sshHost == "" {
		return nil, fmt.Errorf("no SSH target available; set --ssh-remote-host or provide --pve-node")
	}

	sshUser := strings.TrimSpace(rootViperCfg.GetString("ssh_remote_user"))
	if sshUser == "" {
		sshUser = "root"
	}

	return infraSSH.InitializeSshClient(
		sshHost,
		sshUser,
		expandPath(rootViperCfg.GetString("ssh_key")),
		rootViperCfg.GetString("ssh_passphrase"),
		rootViperCfg.GetBool("ssh_use_agent"),
		rootViperCfg.GetUint("ssh_port"),
	)
}

func createProxmoxUserOverSSH(sshClient *goph.Client, cfg proxmoxNewUserOptions) (bool, error) {
	userID := fmt.Sprintf("%s@%s", strings.TrimSpace(cfg.Username), strings.TrimSpace(cfg.Realm))
	exists, err := proxmoxUserExistsOverSSH(sshClient, userID)
	if err != nil {
		return false, fmt.Errorf("check whether proxmox user %s exists: %w", userID, err)
	}
	if exists {
		if !cfg.Force {
			if !promptYesNo(fmt.Sprintf("Proxmox user %s already exists and will be deleted before recreating it. Continue?", userID), false) {
				fmt.Printf("Skipped recreating Proxmox user %s.\n", userID)
				return false, nil
			}
		}
		if err := deleteProxmoxUserOverSSH(sshClient, userID); err != nil {
			return false, fmt.Errorf("delete existing proxmox user %s: %w", userID, err)
		}
	}

	args := []string{"pveum", "user", "add", userID}
	if strings.TrimSpace(cfg.Comment) != "" {
		args = append(args, "--comment", cfg.Comment)
	}
	if strings.TrimSpace(cfg.Password) != "" {
		args = append(args, "--password", cfg.Password)
	}

	_, err = runRemoteQuotedCommand(sshClient, args...)
	if err != nil {
		return false, err
	}
	return true, nil
}

func createProxmoxAPITokenOverSSH(sshClient *goph.Client, cfg proxmoxNewTokenOptions) (proxmoxCreatedToken, error) {
	exists, err := proxmoxTokenExistsOverSSH(sshClient, cfg.UserID, cfg.TokenID)
	if err != nil {
		return proxmoxCreatedToken{}, fmt.Errorf("check whether API token %s!%s exists: %w", cfg.UserID, cfg.TokenID, err)
	}
	if exists {
		fullTokenID := fmt.Sprintf("%s!%s", cfg.UserID, cfg.TokenID)
		if !cfg.Force {
			if !promptYesNo(fmt.Sprintf("API token %s already exists and will be deleted before recreating it. Continue?", fullTokenID), false) {
				fmt.Printf("Skipped recreating Proxmox API token %s.\n", fullTokenID)
				return proxmoxCreatedToken{}, nil
			}
		}
		if err := deleteProxmoxAPITokenOverSSH(sshClient, cfg.UserID, cfg.TokenID); err != nil {
			return proxmoxCreatedToken{}, fmt.Errorf("delete existing API token %s: %w", fullTokenID, err)
		}
	}

	aclArgs := []string{
		"pveum", "aclmod", cfg.ACLPath,
		"-user", cfg.UserID,
		"-role", cfg.Role,
	}
	if _, err := runRemoteQuotedCommand(sshClient, aclArgs...); err != nil {
		return proxmoxCreatedToken{}, fmt.Errorf("apply ACL for %s on %s: %w", cfg.UserID, cfg.ACLPath, err)
	}

	privsepValue := "0"
	if cfg.Privsep {
		privsepValue = "1"
	}

	tokenArgs := []string{
		"pveum", "user", "token", "add", cfg.UserID, cfg.TokenID,
		"--privsep", privsepValue,
		"--output-format", "json",
	}
	if strings.TrimSpace(cfg.Comment) != "" {
		tokenArgs = append(tokenArgs, "--comment", cfg.Comment)
	}

	out, err := runRemoteQuotedCommand(sshClient, tokenArgs...)
	if err != nil {
		return proxmoxCreatedToken{}, fmt.Errorf("create API token %s for %s: %w", cfg.TokenID, cfg.UserID, err)
	}

	createdToken, err := parseCreatedProxmoxToken(cfg.UserID, cfg.TokenID, out)
	if err != nil {
		return proxmoxCreatedToken{}, err
	}

	return createdToken, nil
}

func proxmoxUserExistsOverSSH(sshClient *goph.Client, userID string) (bool, error) {
	out, err := runRemoteQuotedCommand(sshClient, "pveum", "user", "list", "--output-format", "json")
	if err != nil {
		return false, err
	}

	var payload []map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		return false, fmt.Errorf("parse proxmox user list JSON: %w", err)
	}

	for _, item := range payload {
		if strings.TrimSpace(fmt.Sprintf("%v", item["userid"])) == userID {
			return true, nil
		}
	}

	return false, nil
}

func proxmoxTokenExistsOverSSH(sshClient *goph.Client, userID, tokenID string) (bool, error) {
	out, err := runRemoteQuotedCommand(sshClient, "pveum", "user", "token", "list", userID, "--output-format", "json")
	if err != nil {
		return false, err
	}

	var payload []map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		return false, fmt.Errorf("parse proxmox token list JSON: %w", err)
	}

	for _, item := range payload {
		if strings.TrimSpace(fmt.Sprintf("%v", item["tokenid"])) == tokenID {
			return true, nil
		}
	}

	return false, nil
}

func deleteProxmoxUserOverSSH(sshClient *goph.Client, userID string) error {
	_, err := runRemoteQuotedCommand(sshClient, "pveum", "user", "delete", userID)
	return err
}

func deleteProxmoxAPITokenOverSSH(sshClient *goph.Client, userID, tokenID string) error {
	_, err := runRemoteQuotedCommand(sshClient, "pveum", "user", "token", "delete", userID, tokenID)
	return err
}

func runRemoteQuotedCommand(sshClient *goph.Client, args ...string) ([]byte, error) {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}

	command := strings.Join(quoted, " ")
	out, err := sshClient.Run(command)
	if err != nil {
		return nil, formatSSHExecError(err, out)
	}

	return out, nil
}

func parseCreatedProxmoxToken(userID, tokenID string, out []byte) (proxmoxCreatedToken, error) {
	fullTokenID := fmt.Sprintf("%s!%s", userID, tokenID)
	trimmed := strings.TrimSpace(string(out))

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err == nil {
		if value := strings.TrimSpace(fmt.Sprintf("%v", payload["value"])); value != "" && value != "<nil>" {
			if full := strings.TrimSpace(fmt.Sprintf("%v", payload["full-tokenid"])); full != "" && full != "<nil>" {
				fullTokenID = full
			}
			return proxmoxCreatedToken{
				FullTokenID: fullTokenID,
				Secret:      value,
			}, nil
		}
	}

	lines := strings.Split(trimmed, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "value") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
				return proxmoxCreatedToken{
					FullTokenID: fullTokenID,
					Secret:      strings.TrimSpace(parts[1]),
				}, nil
			}
		}
	}

	return proxmoxCreatedToken{}, fmt.Errorf("created token but could not parse the token secret from output: %s", trimmed)
}

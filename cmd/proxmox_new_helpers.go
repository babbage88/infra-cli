package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/babbage88/infra-cli/proxmox"
	infraSSH "github.com/babbage88/infra-cli/ssh"
	"github.com/babbage88/infra-cli/tui"
)

const (
	infraCtlManagerRoleName = "InfraCtlProxmoxManager"
	infraCtlYoloRoleName    = "InfraCtlProxmoxYOLO"
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
	ExpirationDate     string
	DaysValid          int
	ExpireUnix         int64
	Privsep            bool
	WriteDefaultConfig bool
	Force              bool
	Yolo               bool
}

type proxmoxCreatedToken struct {
	FullTokenID string
	Secret      string
}

type proxmoxRoleInfo struct {
	RoleID string
	Privs  []string
}

type proxmoxACLInfo struct {
	Path      string
	Principal string
	RoleID    string
	Propagate bool
}

type proxmoxTokenVerification struct {
	AssignedRoles       []string
	AssignedPrivileges  []string
	DirectChecks        []string
	InferredChecks      []string
	MissingCapabilities []string
}

var (
	proxmoxNewUserFlags  proxmoxNewUserOptions
	proxmoxNewTokenFlags proxmoxNewTokenOptions
)

func initializeProxmoxAdminSSH(pveNode string) (infraSSH.Client, error) {
	sshClient, _, err := initializeRootSSHClient(strings.TrimSpace(pveNode), "root")
	return sshClient, err
}

func createProxmoxUserOverSSH(sshClient infraSSH.Client, cfg proxmoxNewUserOptions) (bool, error) {
	userID := fmt.Sprintf("%s@%s", strings.TrimSpace(cfg.Username), strings.TrimSpace(cfg.Realm))
	exists, err := proxmoxUserExistsOverSSH(sshClient, userID)
	if err != nil {
		return false, fmt.Errorf("check whether proxmox user %s exists: %w", userID, err)
	}
	if exists {
		if !cfg.Force {
			if !tui.YesNo(fmt.Sprintf("Proxmox user %s already exists and will be deleted before recreating it. Continue?", userID), false) {
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

func createProxmoxAPITokenOverSSH(sshClient infraSSH.Client, cfg proxmoxNewTokenOptions) (proxmoxCreatedToken, error) {
	exists, err := proxmoxTokenExistsOverSSH(sshClient, cfg.UserID, cfg.TokenID)
	if err != nil {
		return proxmoxCreatedToken{}, fmt.Errorf("check whether API token %s!%s exists: %w", cfg.UserID, cfg.TokenID, err)
	}
	if exists {
		fullTokenID := fmt.Sprintf("%s!%s", cfg.UserID, cfg.TokenID)
		if !cfg.Force {
			if !tui.YesNo(fmt.Sprintf("API token %s already exists and will be deleted before recreating it. Continue?", fullTokenID), false) {
				fmt.Printf("Skipped recreating Proxmox API token %s.\n", fullTokenID)
				return proxmoxCreatedToken{}, nil
			}
		}
		if err := deleteProxmoxAPITokenOverSSH(sshClient, cfg.UserID, cfg.TokenID); err != nil {
			return proxmoxCreatedToken{}, fmt.Errorf("delete existing API token %s: %w", fullTokenID, err)
		}
	}

	userRoleName, tokenRoleName, err := ensureInfractlRolesForTokenOverSSH(sshClient, cfg)
	if err != nil {
		return proxmoxCreatedToken{}, err
	}
	effectiveRoleName := strings.TrimSpace(tokenRoleName)
	if effectiveRoleName == "" {
		effectiveRoleName = strings.TrimSpace(userRoleName)
	}
	if effectiveRoleName == "" {
		effectiveRoleName = strings.TrimSpace(cfg.Role)
	}

	if userRoleName != "" {
		if err := assignRoleToProxmoxPrincipalOverSSH(sshClient, cfg.ACLPath, "user", cfg.UserID, userRoleName); err != nil {
			return proxmoxCreatedToken{}, fmt.Errorf("apply ACL for user %s on %s: %w", cfg.UserID, cfg.ACLPath, err)
		}
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
	if cfg.ExpireUnix > 0 {
		tokenArgs = append(tokenArgs, "--expire", strconv.FormatInt(cfg.ExpireUnix, 10))
	}

	out, err := runRemoteQuotedCommand(sshClient, tokenArgs...)
	if err != nil {
		if isProxmoxTokenAlreadyExistsError(err) {
			fullTokenID := fmt.Sprintf("%s!%s", cfg.UserID, cfg.TokenID)
			if cfg.Force {
				if err := deleteProxmoxAPITokenOverSSH(sshClient, cfg.UserID, cfg.TokenID); err != nil {
					return proxmoxCreatedToken{}, fmt.Errorf("delete existing API token %s after create reported it already exists: %w", fullTokenID, err)
				}
				cfgCopy := cfg
				return createProxmoxAPITokenOverSSH(sshClient, cfgCopy)
			}
			if !tui.YesNo(fmt.Sprintf("API token %s already exists. Delete and recreate it now?", fullTokenID), false) {
				fmt.Printf("Skipped recreating Proxmox API token %s.\n", fullTokenID)
				return proxmoxCreatedToken{}, nil
			}
			if err := deleteProxmoxAPITokenOverSSH(sshClient, cfg.UserID, cfg.TokenID); err != nil {
				return proxmoxCreatedToken{}, fmt.Errorf("delete existing API token %s: %w", fullTokenID, err)
			}
			cfgCopy := cfg
			cfgCopy.Force = true
			return createProxmoxAPITokenOverSSH(sshClient, cfgCopy)
		}
		return proxmoxCreatedToken{}, fmt.Errorf("create API token %s for %s: %w", cfg.TokenID, cfg.UserID, err)
	}

	createdToken, err := parseCreatedProxmoxToken(cfg.UserID, cfg.TokenID, out)
	if err != nil {
		return proxmoxCreatedToken{}, err
	}

	if effectiveRoleName != "" {
		if err := assignRoleToProxmoxPrincipalOverSSH(sshClient, cfg.ACLPath, "token", createdToken.FullTokenID, effectiveRoleName); err != nil {
			return proxmoxCreatedToken{}, fmt.Errorf("apply ACL for token %s on %s: %w", createdToken.FullTokenID, cfg.ACLPath, err)
		}
	}

	return createdToken, nil
}

func ensureInfractlRolesForTokenOverSSH(sshClient infraSSH.Client, cfg proxmoxNewTokenOptions) (string, string, error) {
	if cfg.Yolo {
		return "", "", nil
	}

	if strings.TrimSpace(cfg.Role) != "" && cfg.Role != infraCtlManagerRoleName {
		if cfg.Privsep {
			return "", cfg.Role, nil
		}
		return cfg.Role, "", nil
	}

	managerPrivs := infractlManagerPrivileges()
	if err := ensureProxmoxRoleOverSSH(sshClient, infraCtlManagerRoleName, managerPrivs); err != nil {
		return "", "", fmt.Errorf("ensure manager role %s: %w", infraCtlManagerRoleName, err)
	}

	if cfg.Privsep {
		return "", infraCtlManagerRoleName, nil
	}

	return infraCtlManagerRoleName, "", nil
}

func infractlManagerPrivileges() []string {
	return []string{
		"Datastore.AllocateSpace",
		"Datastore.Audit",
		"Pool.Allocate",
		"SDN.Use",
		"VM.Allocate",
		"VM.Audit",
		"VM.Clone",
		"VM.Console",
		"VM.Config.CDROM",
		"VM.Config.CPU",
		"VM.Config.Cloudinit",
		"VM.Config.Disk",
		"VM.Config.HWType",
		"VM.Config.Memory",
		"VM.Config.Network",
		"VM.Config.Options",
		"VM.Migrate",
		"VM.PowerMgmt",
	}
}

func ensureProxmoxRoleOverSSH(sshClient infraSSH.Client, roleName string, privs []string) error {
	privs = dedupeAndSortStrings(privs)
	privString := strings.Join(privs, " ")
	if _, err := runRemoteQuotedCommand(sshClient, "pveum", "role", "modify", roleName, "--privs", privString); err == nil {
		return nil
	}

	_, err := runRemoteQuotedCommand(sshClient, "pveum", "role", "add", roleName, "--privs", privString)
	return err
}

func discoverAllAvailablePrivilegesOverSSH(sshClient infraSSH.Client) ([]string, error) {
	roleInfos, err := listProxmoxRolesOverSSH(sshClient)
	if err != nil {
		return nil, err
	}

	var privs []string
	for _, role := range roleInfos {
		privs = append(privs, role.Privs...)
	}

	return dedupeAndSortStrings(privs), nil
}

func assignRoleToProxmoxPrincipalOverSSH(sshClient infraSSH.Client, aclPath, principalKind, principalID, roleName string) error {
	args := []string{"pveum", "aclmod", aclPath, "-role", roleName, "-propagate", "1"}
	switch principalKind {
	case "token":
		args = append(args, "-token", principalID)
	default:
		args = append(args, "-user", principalID)
	}
	_, err := runRemoteQuotedCommand(sshClient, args...)
	return err
}

func proxmoxUserExistsOverSSH(sshClient infraSSH.Client, userID string) (bool, error) {
	out, err := runRemoteQuotedCommand(sshClient, "pveum", "user", "list", "--output-format", "json")
	if err != nil {
		return proxmoxUserExistsFallbackOverSSH(sshClient, userID)
	}

	var payload []map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		trimmed := strings.TrimSpace(string(out))
		if jsonPayload := extractJSONArrayFromOutput(trimmed); jsonPayload != "" {
			if err := json.Unmarshal([]byte(jsonPayload), &payload); err == nil {
				for _, item := range payload {
					if strings.TrimSpace(fmt.Sprintf("%v", item["userid"])) == userID {
						return true, nil
					}
				}
				return false, nil
			}
		}
		return proxmoxUserExistsFallbackOverSSH(sshClient, userID)
	}

	for _, item := range payload {
		if strings.TrimSpace(fmt.Sprintf("%v", item["userid"])) == userID {
			return true, nil
		}
	}

	return false, nil
}

func proxmoxUserExistsFallbackOverSSH(sshClient infraSSH.Client, userID string) (bool, error) {
	out, err := runRemoteQuotedCommand(sshClient, "pveum", "user", "list")
	if err != nil {
		return false, err
	}

	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 || fields[0] == "userid" {
			continue
		}
		if fields[0] == userID {
			return true, nil
		}
	}

	return false, nil
}

func proxmoxTokenExistsOverSSH(sshClient infraSSH.Client, userID, tokenID string) (bool, error) {
	out, err := runRemoteQuotedCommand(sshClient, "pveum", "user", "token", "list", userID, "--output-format", "json")
	if err != nil {
		return proxmoxTokenExistsFallbackOverSSH(sshClient, userID, tokenID)
	}

	var payload []map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		return proxmoxTokenExistsFallbackOverSSH(sshClient, userID, tokenID)
	}

	for _, item := range payload {
		if strings.TrimSpace(fmt.Sprintf("%v", item["tokenid"])) == tokenID {
			return true, nil
		}
	}

	return false, nil
}

func proxmoxTokenExistsFallbackOverSSH(sshClient infraSSH.Client, userID, tokenID string) (bool, error) {
	out, err := runRemoteQuotedCommand(sshClient, "pveum", "user", "token", "list", userID)
	if err == nil {
		if proxmoxTokenListContainsToken(string(out), tokenID) {
			return true, nil
		}
		return false, nil
	}

	out, err = runRemoteQuotedCommand(sshClient, "pveum", "user", "token", "permissions", userID+"!"+tokenID)
	if err == nil {
		return true, nil
	}

	rawErr := strings.ToLower(err.Error())
	if strings.Contains(rawErr, "not exist") || strings.Contains(rawErr, "does not exist") || strings.Contains(rawErr, "no such") {
		return false, nil
	}

	return false, fmt.Errorf("fallback token existence checks failed: %w", err)
}

func proxmoxTokenListContainsToken(output, tokenID string) bool {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		if fields[0] == tokenID {
			return true
		}
	}
	return false
}

func deleteProxmoxUserOverSSH(sshClient infraSSH.Client, userID string) error {
	_, err := runRemoteQuotedCommand(sshClient, "pveum", "user", "delete", userID)
	return err
}

func deleteProxmoxAPITokenOverSSH(sshClient infraSSH.Client, userID, tokenID string) error {
	_, err := runRemoteQuotedCommand(sshClient, "pveum", "user", "token", "delete", userID, tokenID)
	return err
}

func runRemoteQuotedCommand(sshClient infraSSH.Client, args ...string) ([]byte, error) {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}

	command := infraSSH.WithDefaultTERM(strings.Join(quoted, " "))
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

	if jsonPayload := extractJSONObjectFromOutput(trimmed); jsonPayload != "" {
		if err := json.Unmarshal([]byte(jsonPayload), &payload); err == nil {
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
	}

	lines := strings.Split(trimmed, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if err := json.Unmarshal([]byte(line), &payload); err == nil {
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

func extractJSONObjectFromOutput(output string) string {
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return strings.TrimSpace(output[start : end+1])
}

func extractJSONArrayFromOutput(output string) string {
	start := strings.Index(output, "[")
	end := strings.LastIndex(output, "]")
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return strings.TrimSpace(output[start : end+1])
}

func isProxmoxTokenAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "token already exists") ||
		(strings.Contains(lower, "already exists") && strings.Contains(lower, "tokenid"))
}

func listProxmoxRolesOverSSH(sshClient infraSSH.Client) ([]proxmoxRoleInfo, error) {
	out, err := runRemoteQuotedCommand(sshClient, "pveum", "role", "list", "--output-format", "json")
	if err != nil {
		return listProxmoxRolesFallbackOverSSH(sshClient)
	}

	var payload []map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		return listProxmoxRolesFallbackOverSSH(sshClient)
	}

	roles := make([]proxmoxRoleInfo, 0, len(payload))
	for _, item := range payload {
		roleID := strings.TrimSpace(fmt.Sprintf("%v", item["roleid"]))
		if roleID == "" {
			continue
		}
		privs := splitPrivilegeString(fmt.Sprintf("%v", item["privs"]))
		roles = append(roles, proxmoxRoleInfo{RoleID: roleID, Privs: privs})
	}

	return roles, nil
}

func listProxmoxACLsOverSSH(sshClient infraSSH.Client) ([]proxmoxACLInfo, error) {
	out, err := runRemoteQuotedCommand(sshClient, "pveum", "acl", "list", "--output-format", "json")
	if err != nil {
		return listProxmoxACLsFallbackOverSSH(sshClient)
	}

	var payload []map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		return listProxmoxACLsFallbackOverSSH(sshClient)
	}

	acls := make([]proxmoxACLInfo, 0, len(payload))
	for _, item := range payload {
		principal := strings.TrimSpace(fmt.Sprintf("%v", item["ugid"]))
		if principal == "" {
			continue
		}
		roleID := strings.TrimSpace(fmt.Sprintf("%v", item["roleid"]))
		path := strings.TrimSpace(fmt.Sprintf("%v", item["path"]))
		propagate := strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", item["propagate"])), "1") ||
			strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", item["propagate"])), "true")
		acls = append(acls, proxmoxACLInfo{
			Path:      path,
			Principal: principal,
			RoleID:    roleID,
			Propagate: propagate,
		})
	}

	return acls, nil
}

func listProxmoxRolesFallbackOverSSH(sshClient infraSSH.Client) ([]proxmoxRoleInfo, error) {
	out, err := runRemoteQuotedCommand(sshClient, "pveum", "role", "list")
	if err != nil {
		return nil, fmt.Errorf("list proxmox roles via plain-text fallback: %w", err)
	}

	roles := make([]proxmoxRoleInfo, 0)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "ROLEID") ||
			strings.HasPrefix(upper, "USAGE:") ||
			strings.HasPrefix(strings.ToLower(line), "user config -") ||
			strings.HasPrefix(line, "400 ") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		roleID := strings.TrimSpace(fields[0])
		if roleID == "" {
			continue
		}

		privs := splitPrivilegeString(strings.Join(fields[1:], " "))
		roles = append(roles, proxmoxRoleInfo{RoleID: roleID, Privs: privs})
	}

	if len(roles) == 0 {
		return nil, fmt.Errorf("parse proxmox role list fallback output: no roles found")
	}

	return roles, nil
}

func listProxmoxACLsFallbackOverSSH(sshClient infraSSH.Client) ([]proxmoxACLInfo, error) {
	out, err := runRemoteQuotedCommand(sshClient, "pveum", "acl", "list")
	if err != nil {
		return nil, fmt.Errorf("list proxmox ACLs via plain-text fallback: %w", err)
	}

	acls := make([]proxmoxACLInfo, 0)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "PATH") || strings.HasPrefix(upper, "ACLPATH") || strings.HasPrefix(upper, "USAGE:") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		propagate := false
		lastField := strings.ToLower(fields[len(fields)-1])
		if lastField == "0" || lastField == "1" || lastField == "true" || lastField == "false" {
			propagate = lastField == "1" || lastField == "true"
			fields = fields[:len(fields)-1]
		}
		if len(fields) < 3 {
			continue
		}

		acls = append(acls, proxmoxACLInfo{
			Path:      strings.TrimSpace(fields[0]),
			Principal: strings.TrimSpace(fields[1]),
			RoleID:    strings.TrimSpace(fields[2]),
			Propagate: propagate,
		})
	}

	if len(acls) == 0 {
		return nil, fmt.Errorf("parse proxmox ACL list fallback output: no ACLs found")
	}

	return acls, nil
}

func inspectProxmoxAuthOverSSH(sshClient infraSSH.Client, userID, tokenFullID string) ([]string, []string, error) {
	roleInfos, err := listProxmoxRolesOverSSH(sshClient)
	if err != nil {
		return nil, nil, err
	}
	acls, err := listProxmoxACLsOverSSH(sshClient)
	if err != nil {
		return nil, nil, err
	}

	roleMap := make(map[string][]string, len(roleInfos))
	for _, role := range roleInfos {
		roleMap[role.RoleID] = role.Privs
	}

	roleSet := make(map[string]struct{})
	privSet := make(map[string]struct{})
	for _, acl := range acls {
		if acl.Principal != userID && acl.Principal != tokenFullID {
			continue
		}
		roleSet[acl.RoleID] = struct{}{}
		for _, priv := range roleMap[acl.RoleID] {
			privSet[priv] = struct{}{}
		}
	}

	return mapKeysSorted(roleSet), mapKeysSorted(privSet), nil
}

func verifyProxmoxTokenCoversInfraCtlCommands(sshClient infraSSH.Client, cfg proxmoxNewTokenOptions, createdToken proxmoxCreatedToken) (*proxmoxTokenVerification, error) {
	assignedRoles, assignedPrivs, err := inspectProxmoxAuthOverSSH(sshClient, cfg.UserID, createdToken.FullTokenID)
	if err != nil {
		return nil, fmt.Errorf("inspect assigned ACLs and roles: %w", err)
	}

	hostURL := strings.TrimSpace(rootViperCfg.GetString("proxmox_api_url"))
	if hostURL == "" {
		hostURL = strings.TrimSpace(rootViperCfg.GetString("proxmox_host"))
	}
	if hostURL == "" {
		hostURL = defaultProxmoxHostURL(cfg.PveNode)
	}

	tokenID, secret, err := proxmox.ParseAPIToken(fmt.Sprintf("%s=%s", createdToken.FullTokenID, createdToken.Secret))
	if err != nil {
		return nil, fmt.Errorf("parse created token for verification: %w", err)
	}
	client, err := proxmox.NewClientToken(hostURL, tokenID, secret, true)
	if err != nil {
		return nil, fmt.Errorf("create proxmox client for verification: %w", err)
	}

	verification := &proxmoxTokenVerification{
		AssignedRoles:      assignedRoles,
		AssignedPrivileges: assignedPrivs,
	}
	if strings.TrimSpace(cfg.ACLPath) != "" && strings.TrimSpace(cfg.ACLPath) != "/" {
		verification.MissingCapabilities = append(
			verification.MissingCapabilities,
			fmt.Sprintf("ACL path %s is narrower than /, so verification does not imply cluster-wide access for every infractl proxmox subcommand", cfg.ACLPath),
		)
	}

	ctx := context.Background()
	if _, err := client.ListVMs(ctx, cfg.PveNode, false); err != nil {
		verification.MissingCapabilities = append(verification.MissingCapabilities, fmt.Sprintf("proxmox vm list direct API smoke test failed: %v", err))
	} else {
		verification.DirectChecks = append(verification.DirectChecks, "proxmox vm list")
	}

	if storages, err := client.ListNodeStorage(ctx, cfg.PveNode); err != nil {
		verification.MissingCapabilities = append(verification.MissingCapabilities, fmt.Sprintf("proxmox lxc create storage lookup failed: %v", err))
	} else {
		verification.DirectChecks = append(verification.DirectChecks, fmt.Sprintf("proxmox lxc create storage lookup (%d storages)", len(storages)))
	}

	if templates, err := client.ListLxcTemplates(ctx, cfg.PveNode); err != nil {
		verification.MissingCapabilities = append(verification.MissingCapabilities, fmt.Sprintf("proxmox lxc create template lookup failed: %v", err))
	} else {
		verification.DirectChecks = append(verification.DirectChecks, fmt.Sprintf("proxmox lxc create template lookup (%d templates)", len(templates)))
	}

	if cfg.Yolo && !cfg.Privsep && cfg.UserID == "root@pam" {
		verification.InferredChecks = append(verification.InferredChecks,
			"proxmox vm get (inherited from root@pam via non-privsep token)",
			"proxmox vm start (inherited from root@pam via non-privsep token)",
			"proxmox vm set (inherited from root@pam via non-privsep token)",
			"proxmox vm create (inherited from root@pam via non-privsep token)",
			"proxmox lxc create (inherited from root@pam via non-privsep token)",
			"proxmox lxc batch (inherited from root@pam via non-privsep token)",
		)
		return verification, nil
	}

	verifyCapability := func(command string, required []string) {
		missing := missingPrivileges(assignedPrivs, required)
		if len(missing) == 0 {
			verification.InferredChecks = append(verification.InferredChecks, fmt.Sprintf("%s (via privileges)", command))
			return
		}
		verification.MissingCapabilities = append(verification.MissingCapabilities, fmt.Sprintf("%s missing privileges: %s", command, strings.Join(missing, ", ")))
	}

	verifyCapability("proxmox vm get", []string{"VM.Audit"})
	verifyCapability("proxmox vm start", []string{"VM.PowerMgmt"})
	verifyCapability("proxmox vm set", []string{"VM.Config.CPU", "VM.Config.Memory", "VM.Config.Options"})
	verifyCapability("proxmox vm create", []string{"VM.Allocate", "VM.Config.CPU", "VM.Config.Memory", "VM.Config.Options"})
	verifyCapability("proxmox lxc create", []string{"VM.Allocate", "VM.Config.CPU", "VM.Config.Memory", "VM.Config.Network", "VM.Config.Options", "Datastore.Audit", "Datastore.AllocateSpace"})
	verifyCapability("proxmox lxc batch", []string{"VM.Allocate", "VM.Config.CPU", "VM.Config.Memory", "VM.Config.Network", "VM.Config.Options", "Datastore.Audit", "Datastore.AllocateSpace"})

	return verification, nil
}

func splitPrivilegeString(value string) []string {
	value = strings.ReplaceAll(strings.TrimSpace(value), ",", " ")
	fields := strings.Fields(value)
	filtered := make([]string, 0, len(fields))
	for _, field := range fields {
		if isLikelyProxmoxPrivilege(field) {
			filtered = append(filtered, field)
		}
	}
	return dedupeAndSortStrings(filtered)
}

func isLikelyProxmoxPrivilege(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || !strings.Contains(value, ".") {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' {
			continue
		}
		return false
	}
	return true
}

func dedupeAndSortStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}

func mapKeysSorted[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func missingPrivileges(have []string, required []string) []string {
	haveSet := make(map[string]struct{}, len(have))
	for _, value := range have {
		haveSet[value] = struct{}{}
	}

	missing := make([]string, 0)
	for _, value := range required {
		if _, ok := haveSet[value]; ok {
			continue
		}
		missing = append(missing, value)
	}

	return dedupeAndSortStrings(missing)
}

func printProxmoxTokenVerification(verification *proxmoxTokenVerification) {
	if verification == nil {
		return
	}

	fmt.Println("Verification summary:")
	if len(verification.AssignedRoles) > 0 {
		fmt.Printf("  Assigned roles: %s\n", strings.Join(verification.AssignedRoles, ", "))
	}
	if len(verification.AssignedPrivileges) > 0 {
		fmt.Printf("  Assigned privileges: %s\n", strings.Join(verification.AssignedPrivileges, ", "))
	}
	for _, check := range verification.DirectChecks {
		fmt.Printf("  Direct check OK: %s\n", check)
	}
	for _, check := range verification.InferredChecks {
		fmt.Printf("  Permission coverage OK: %s\n", check)
	}
	for _, check := range verification.MissingCapabilities {
		fmt.Printf("  Missing or failed: %s\n", check)
	}
}

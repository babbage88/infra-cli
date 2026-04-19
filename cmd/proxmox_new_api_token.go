package cmd

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/babbage88/infra-cli/tui"
	"github.com/spf13/cobra"
)

var proxmoxNewAPITokenCmd = &cobra.Command{
	Use:   "api-token",
	Short: "Create a new Proxmox API token for a user via SSH on a Proxmox node",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := proxmoxNewTokenFlags
		if cfg.Yolo {
			fmt.Println("YOLO mode enabled: this will create a non-privsep token for root@pam so it inherits root permissions.")
		}
		if err := promptForMissingProxmoxNewTokenValues(&cfg); err != nil {
			return err
		}
		if err := resolveProxmoxNewTokenExpiration(&cfg); err != nil {
			return err
		}

		sshClient, err := initializeProxmoxAdminSSH(cfg.PveNode)
		if err != nil {
			return err
		}
		defer sshClient.Close()

		createdToken, err := createProxmoxAPITokenOverSSH(sshClient, cfg)
		if err != nil {
			return err
		}
		if strings.TrimSpace(createdToken.FullTokenID) == "" {
			return nil
		}

		fmt.Printf("Created Proxmox API token %s.\n", createdToken.FullTokenID)
		fmt.Printf("Token secret: %s\n", createdToken.Secret)

		verification, err := verifyProxmoxTokenCoversInfraCtlCommands(sshClient, cfg, createdToken)
		if err != nil {
			return fmt.Errorf("post-create token verification failed: %w", err)
		}
		printProxmoxTokenVerification(verification)

		if cfg.WriteDefaultConfig {
			encodedTokenID := base64.StdEncoding.EncodeToString([]byte(createdToken.FullTokenID))
			encodedSecret := base64.StdEncoding.EncodeToString([]byte(createdToken.Secret))
			targetPath, err := writeDefaultRootConfigValues(map[string]string{
				"proxmox_api_token":  encodedTokenID,
				"proxmox_api_secret": encodedSecret,
			})
			if err != nil {
				return fmt.Errorf("write proxmox credentials to default config: %w", err)
			}
			fmt.Printf("Wrote base64-encoded proxmox_api_token and proxmox_api_secret to %s.\n", targetPath)
		}

		return nil
	},
}

func promptForMissingProxmoxNewTokenValues(cfg *proxmoxNewTokenOptions) error {
	if strings.TrimSpace(cfg.PveNode) == "" {
		cfg.PveNode = strings.TrimSpace(rootViperCfg.GetString("ssh_remote_host"))
	}
	if strings.TrimSpace(cfg.PveNode) == "" {
		cfg.PveNode = tui.InputWithExample("Proxmox node", "pve01", rootViperCfg.GetString("ssh_remote_host"))
	}
	if cfg.Yolo {
		cfg.Username = "root"
		cfg.Realm = "pam"
		cfg.UserID = "root@pam"
		cfg.ACLPath = "/"
		cfg.Privsep = false
		cfg.Role = ""
		if strings.TrimSpace(cfg.TokenID) == "" {
			cfg.TokenID = tui.InputWithExample("Proxmox API token ID", "infractl-yolo-unsafe", "infractl-yolo-unsafe")
		}
		if strings.TrimSpace(cfg.Comment) == "" {
			cfg.Comment = tui.OptionalInput("Token comment", "infractlYOLOunsafe")
		}
		return nil
	}
	if strings.TrimSpace(cfg.UserID) == "" {
		if strings.TrimSpace(cfg.Username) == "" {
			cfg.Username = tui.InputWithExample("Proxmox username", "infractl", "")
		}
		if strings.TrimSpace(cfg.Realm) == "" {
			cfg.Realm = tui.InputWithExample("Proxmox realm", "pve", "pve")
		}
		cfg.UserID = fmt.Sprintf("%s@%s", strings.TrimSpace(cfg.Username), strings.TrimSpace(cfg.Realm))
	}
	if strings.TrimSpace(cfg.TokenID) == "" {
		cfg.TokenID = tui.InputWithExample("Proxmox API token ID", "infractl-cli", "infractl-cli")
	}
	if strings.TrimSpace(cfg.Comment) == "" {
		cfg.Comment = tui.OptionalInput("Token comment", "Created by infractl")
	}
	if strings.TrimSpace(cfg.Role) == "" {
		cfg.Role = tui.InputWithExample("Role to assign for VM/LXC management", infraCtlManagerRoleName, infraCtlManagerRoleName)
	}
	if strings.TrimSpace(cfg.ACLPath) == "" {
		cfg.ACLPath = tui.InputWithExample("ACL path", "/", "/")
	}

	return nil
}

func resolveProxmoxNewTokenExpiration(cfg *proxmoxNewTokenOptions) error {
	expirationDate := strings.TrimSpace(cfg.ExpirationDate)
	if expirationDate != "" && cfg.DaysValid > 0 {
		return fmt.Errorf("use either --expiration-date or --days-valid, not both")
	}
	if cfg.DaysValid < 0 {
		return fmt.Errorf("--days-valid must be zero or greater")
	}
	if cfg.DaysValid > 0 {
		cfg.ExpireUnix = time.Now().AddDate(0, 0, cfg.DaysValid).Unix()
		return nil
	}
	if expirationDate == "" {
		return nil
	}

	if parsed, err := time.Parse(time.RFC3339, expirationDate); err == nil {
		cfg.ExpireUnix = parsed.Unix()
		return nil
	}

	parsedDate, err := time.ParseInLocation("2006-01-02", expirationDate, time.Local)
	if err != nil {
		return fmt.Errorf("parse --expiration-date %q: use YYYY-MM-DD or RFC3339, for example 2026-05-19 or 2026-05-19T23:59:59-05:00", expirationDate)
	}
	cfg.ExpireUnix = parsedDate.AddDate(0, 0, 1).Add(-time.Second).Unix()
	return nil
}

func init() {
	proxmoxNewCmd.AddCommand(proxmoxNewAPITokenCmd)

	proxmoxNewAPITokenCmd.Flags().StringVar(&proxmoxNewTokenFlags.PveNode, "pve-node", "", "Proxmox node to connect to over SSH")
	proxmoxNewAPITokenCmd.Flags().StringVar(&proxmoxNewTokenFlags.UserID, "userid", "", "Proxmox user in user@realm form")
	proxmoxNewAPITokenCmd.Flags().StringVar(&proxmoxNewTokenFlags.Username, "username", "", "Proxmox username without the realm suffix")
	proxmoxNewAPITokenCmd.Flags().StringVar(&proxmoxNewTokenFlags.Realm, "realm", "pve", "Proxmox realm")
	proxmoxNewAPITokenCmd.Flags().StringVar(&proxmoxNewTokenFlags.TokenID, "token-id", "", "API token ID")
	proxmoxNewAPITokenCmd.Flags().StringVar(&proxmoxNewTokenFlags.Comment, "comment", "", "Comment to attach to the API token")
	proxmoxNewAPITokenCmd.Flags().StringVar(&proxmoxNewTokenFlags.Role, "role", infraCtlManagerRoleName, "Role to apply for VM and LXC management")
	proxmoxNewAPITokenCmd.Flags().StringVar(&proxmoxNewTokenFlags.ACLPath, "acl-path", "/", "ACL path to grant the role on")
	proxmoxNewAPITokenCmd.Flags().StringVar(&proxmoxNewTokenFlags.ExpirationDate, "expiration-date", "", "Date/time when the API token should expire, as YYYY-MM-DD or RFC3339")
	proxmoxNewAPITokenCmd.Flags().StringVar(&proxmoxNewTokenFlags.ExpirationDate, "exiriation-date", "", "Deprecated typo alias for --expiration-date")
	_ = proxmoxNewAPITokenCmd.Flags().MarkHidden("exiriation-date")
	proxmoxNewAPITokenCmd.Flags().IntVar(&proxmoxNewTokenFlags.DaysValid, "days-valid", 0, "Number of days the API token should remain valid")
	proxmoxNewAPITokenCmd.Flags().BoolVar(&proxmoxNewTokenFlags.Privsep, "privsep", false, "Create the token with privilege separation enabled")
	proxmoxNewAPITokenCmd.Flags().BoolVar(&proxmoxNewTokenFlags.WriteDefaultConfig, "write-default-config", false, "Write the new token ID and secret to the default config file as base64 values")
	proxmoxNewAPITokenCmd.Flags().BoolVar(&proxmoxNewTokenFlags.Force, "force", false, "Delete and recreate the API token without prompting if it already exists")
	proxmoxNewAPITokenCmd.Flags().BoolVar(&proxmoxNewTokenFlags.Yolo, "yolo", false, "Grant full discovered cluster privileges to both the user and token")
}

package cmd

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var proxmoxNewAPITokenCmd = &cobra.Command{
	Use:   "api-token",
	Short: "Create a new Proxmox API token for a user via SSH on a Proxmox node",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := proxmoxNewTokenFlags
		if err := promptForMissingProxmoxNewTokenValues(&cfg); err != nil {
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

		fmt.Printf("Created Proxmox API token %s.\n", createdToken.FullTokenID)
		fmt.Printf("Token secret: %s\n", createdToken.Secret)

		if cfg.WriteDefaultConfig {
			encodedTokenID := base64.StdEncoding.EncodeToString([]byte(createdToken.FullTokenID))
			encodedSecret := base64.StdEncoding.EncodeToString([]byte(createdToken.Secret))
			if err := writeRootConfigValues(map[string]string{
				"proxmox_api_token":  encodedTokenID,
				"proxmox_api_secret": encodedSecret,
			}); err != nil {
				return fmt.Errorf("write proxmox credentials to default config: %w", err)
			}
			fmt.Println("Wrote base64-encoded proxmox_api_token and proxmox_api_secret to your default config.")
		}

		return nil
	},
}

func promptForMissingProxmoxNewTokenValues(cfg *proxmoxNewTokenOptions) error {
	if strings.TrimSpace(cfg.PveNode) == "" {
		cfg.PveNode = strings.TrimSpace(rootViperCfg.GetString("ssh_remote_host"))
	}
	if strings.TrimSpace(cfg.PveNode) == "" {
		cfg.PveNode = promptInputWithExample("Proxmox node", "pve01", rootViperCfg.GetString("ssh_remote_host"))
	}
	if strings.TrimSpace(cfg.UserID) == "" {
		if strings.TrimSpace(cfg.Username) == "" {
			cfg.Username = promptInputWithExample("Proxmox username", "infractl", "")
		}
		if strings.TrimSpace(cfg.Realm) == "" {
			cfg.Realm = promptInputWithExample("Proxmox realm", "pve", "pve")
		}
		cfg.UserID = fmt.Sprintf("%s@%s", strings.TrimSpace(cfg.Username), strings.TrimSpace(cfg.Realm))
	}
	if strings.TrimSpace(cfg.TokenID) == "" {
		cfg.TokenID = promptInputWithExample("Proxmox API token ID", "infractl-cli", "infractl-cli")
	}
	if strings.TrimSpace(cfg.Comment) == "" {
		cfg.Comment = promptOptionalInput("Token comment", "Created by infractl")
	}
	if strings.TrimSpace(cfg.Role) == "" {
		cfg.Role = promptInputWithExample("Role to assign for VM/LXC management", "PVEAdmin", "PVEAdmin")
	}
	if strings.TrimSpace(cfg.ACLPath) == "" {
		cfg.ACLPath = promptInputWithExample("ACL path", "/", "/")
	}

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
	proxmoxNewAPITokenCmd.Flags().StringVar(&proxmoxNewTokenFlags.Role, "role", "PVEAdmin", "Role to apply for VM and LXC management")
	proxmoxNewAPITokenCmd.Flags().StringVar(&proxmoxNewTokenFlags.ACLPath, "acl-path", "/", "ACL path to grant the role on")
	proxmoxNewAPITokenCmd.Flags().BoolVar(&proxmoxNewTokenFlags.Privsep, "privsep", false, "Create the token with privilege separation enabled")
	proxmoxNewAPITokenCmd.Flags().BoolVar(&proxmoxNewTokenFlags.WriteDefaultConfig, "write-default-config", false, "Write the new token ID and secret to the default config file as base64 values")
}

package cmd

import (
	"fmt"
	"strings"

	"github.com/babbage88/infra-cli/tui"
	"github.com/spf13/cobra"
)

type proxmoxAuthInspectOptions struct {
	PveNode  string
	UserID   string
	Username string
	Realm    string
	TokenID  string
}

var proxmoxAuthInspectFlags proxmoxAuthInspectOptions

var proxmoxAuthInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect ACLs, roles, and inferred privileges for a Proxmox user or token",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := proxmoxAuthInspectFlags
		if strings.TrimSpace(cfg.PveNode) == "" {
			cfg.PveNode = strings.TrimSpace(rootViperCfg.GetString("ssh_remote_host"))
		}
		if strings.TrimSpace(cfg.PveNode) == "" {
			cfg.PveNode = tui.InputWithExample("Proxmox node", "pve01", rootViperCfg.GetString("ssh_remote_host"))
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

		tokenFullID := ""
		if strings.TrimSpace(cfg.TokenID) != "" {
			tokenFullID = fmt.Sprintf("%s!%s", cfg.UserID, cfg.TokenID)
		}

		sshClient, err := initializeProxmoxAdminSSH(cfg.PveNode)
		if err != nil {
			return err
		}
		defer sshClient.Close()

		roles, privs, err := inspectProxmoxAuthOverSSH(sshClient, cfg.UserID, tokenFullID)
		if err != nil {
			return err
		}

		fmt.Printf("Inspection target: %s\n", cfg.UserID)
		if tokenFullID != "" {
			fmt.Printf("Token target: %s\n", tokenFullID)
		}
		if len(roles) > 0 {
			fmt.Printf("Roles: %s\n", strings.Join(roles, ", "))
		} else {
			fmt.Println("Roles: none found")
		}
		if len(privs) > 0 {
			fmt.Printf("Privileges: %s\n", strings.Join(privs, ", "))
		} else {
			fmt.Println("Privileges: none found")
		}

		return nil
	},
}

func init() {
	proxmoxAuthCmd.AddCommand(proxmoxAuthInspectCmd)

	proxmoxAuthInspectCmd.Flags().StringVar(&proxmoxAuthInspectFlags.PveNode, "pve-node", "", "Proxmox node to connect to over SSH")
	proxmoxAuthInspectCmd.Flags().StringVar(&proxmoxAuthInspectFlags.UserID, "userid", "", "Proxmox user in user@realm form")
	proxmoxAuthInspectCmd.Flags().StringVar(&proxmoxAuthInspectFlags.Username, "username", "", "Proxmox username without the realm suffix")
	proxmoxAuthInspectCmd.Flags().StringVar(&proxmoxAuthInspectFlags.Realm, "realm", "pve", "Proxmox realm")
	proxmoxAuthInspectCmd.Flags().StringVar(&proxmoxAuthInspectFlags.TokenID, "token-id", "", "Optional token ID to inspect alongside the user")
}

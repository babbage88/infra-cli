package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var proxmoxNewPVEUserCmd = &cobra.Command{
	Use:   "pve-user",
	Short: "Create a new Proxmox user via SSH on a Proxmox node",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := proxmoxNewUserFlags
		if err := promptForMissingProxmoxNewUserValues(&cfg); err != nil {
			return err
		}

		sshClient, err := initializeProxmoxAdminSSH(cfg.PveNode)
		if err != nil {
			return err
		}
		defer sshClient.Close()

		created, err := createProxmoxUserOverSSH(sshClient, cfg)
		if err != nil {
			return err
		}
		if !created {
			return nil
		}
		fmt.Printf("Created Proxmox user %s@%s on %s.\n", cfg.Username, cfg.Realm, cfg.PveNode)
		return nil
	},
}

func promptForMissingProxmoxNewUserValues(cfg *proxmoxNewUserOptions) error {
	if strings.TrimSpace(cfg.PveNode) == "" {
		cfg.PveNode = strings.TrimSpace(rootViperCfg.GetString("ssh_remote_host"))
	}
	if strings.TrimSpace(cfg.PveNode) == "" {
		cfg.PveNode = promptInputWithExample("Proxmox node", "pve01", rootViperCfg.GetString("ssh_remote_host"))
	}
	if strings.TrimSpace(cfg.Username) == "" {
		cfg.Username = promptInputWithExample("New Proxmox username", "infractl", "")
	}
	if strings.TrimSpace(cfg.Realm) == "" {
		cfg.Realm = promptInputWithExample("Proxmox realm", "pve", "pve")
	}
	if strings.TrimSpace(cfg.Comment) == "" {
		cfg.Comment = promptOptionalInput("Proxmox user comment", "Created by infractl")
	}
	if cfg.Password == "" && promptYesNo("Set a password for this Proxmox user?", false) {
		cfg.Password = promptPasswordWithExample("Proxmox user password", "correct-horse-battery-staple", "")
	}

	return nil
}

func init() {
	proxmoxNewCmd.AddCommand(proxmoxNewPVEUserCmd)

	proxmoxNewPVEUserCmd.Flags().StringVar(&proxmoxNewUserFlags.PveNode, "pve-node", "", "Proxmox node to connect to over SSH")
	proxmoxNewPVEUserCmd.Flags().StringVar(&proxmoxNewUserFlags.Username, "username", "", "New Proxmox username without the realm suffix")
	proxmoxNewPVEUserCmd.Flags().StringVar(&proxmoxNewUserFlags.Realm, "realm", "pve", "Proxmox realm")
	proxmoxNewPVEUserCmd.Flags().StringVar(&proxmoxNewUserFlags.Comment, "comment", "", "Comment to attach to the Proxmox user")
	proxmoxNewPVEUserCmd.Flags().StringVar(&proxmoxNewUserFlags.Password, "password", "", "Optional password for the new Proxmox user")
	proxmoxNewPVEUserCmd.Flags().BoolVar(&proxmoxNewUserFlags.Force, "force", false, "Delete and recreate the user without prompting if it already exists")
}

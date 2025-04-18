package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/babbage88/infra-cli/proxmox"
	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var createBatchCmd = &cobra.Command{
	Use:     "create-batch",
	Aliases: []string{"create-lxc-batch", "batch"},
	Short:   "Create multiple LXC containers from a YAML file",
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		localViper := viper.New()
		cfgFile, _ := cmd.Flags().GetString("config-file")
		if cfgFile != "" {
			err := loadProxmoxConfigFile(cfgFile, localViper)
			if err != nil {
				log.Fatalf("Failed to load config: %v", err)
			}
		}

		// Read the YAML file containing the batch of containers
		filePath, _ := cmd.Flags().GetString("file")
		if filePath == "" {
			log.Fatal("No YAML file provided")
		}

		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalf("Failed to read file: %v", err)
		}

		var lxcContainers []proxmox.LxcContainer
		if err := yaml.Unmarshal(fileContent, &lxcContainers); err != nil {
			log.Fatalf("Failed to parse YAML: %v", err)
		}

		// Create each container
		for _, lxc := range lxcContainers {
			fmt.Printf("Creating LXC container %d...\n", lxc.VmId)
			err := proxmox.CreateLXCContainer(localViper.GetString("host_url"), localViper.GetString("api_token"), lxc.Node, lxc.ToFormParams())
			if err != nil {
				log.Fatalf("Error creating container: %v", err)
			}
			fmt.Println("Container created successfully")
		}
	},
}

func init() {
	proxmoxLxcSubCmd.AddCommand(createBatchCmd)

	createBatchCmd.Flags().StringVar(&configFilePath, "config-file", "", "Path to YAML config file containing proxmox lxc info")
	createBatchCmd.Flags().String("file", "", "Path to the YAML file containing batch LXC container configuration")
}

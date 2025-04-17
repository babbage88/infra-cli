package cmd

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/babbage88/infra-cli/ssh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cmdToRun       string
	hostConnMap    = make(map[string]string)
	configFilePath string
	mergedHostMap  = make(map[string][]string)
)

var clusterSsh = &cobra.Command{
	Use:     "cluster-ssh",
	Aliases: []string{"cssh"},
	Short:   "Execute SSH commands concurrently across multiple hosts",
	RunE: func(cmd *cobra.Command, args []string) error {
		start := time.Now()

		// Step 1: Load config file into mergedHostMap
		configLoaded := false
		if configFilePath != "" {
			if err := loadHostMapFromConfig(configFilePath, mergedHostMap); err != nil {
				return err
			}
			configLoaded = true
		}

		// Step 2: Merge flags into mergedHostMap
		flagProvided := len(hostConnMap) > 0
		if flagProvided {
			mergeHostMapFromFlags(hostConnMap, mergedHostMap)
		}

		// Step 3: Deduplicate only if both were provided
		if configLoaded && flagProvided {
			dedupeHostMap(mergedHostMap)
		}

		if len(mergedHostMap) == 0 {
			return fmt.Errorf("no hosts provided from --hostnames or --config-file")
		}
		if cmdToRun == "" {
			return fmt.Errorf("--cmd is required")
		}

		// Execute SSH commands concurrently
		var wg sync.WaitGroup
		results := make(chan string, 50)
		totalHosts := 0

		for user, hosts := range mergedHostMap {
			for _, host := range hosts {
				totalHosts++
				wg.Add(1)
				go func(user, host string) {
					defer wg.Done()
					agent, err := ssh.NewRemoteAppDeploymentAgentWithSshKey(
						host, user, "", "",
						rootViperCfg.GetString("ssh_key"),
						rootViperCfg.GetString("ssh_passphrase"),
						nil,
						rootViperCfg.GetBool("ssh_use_agent"),
						rootViperCfg.GetUint("ssh_port"),
					)
					if err != nil {
						results <- fmt.Sprintf("[%s@%s] connection failed: %v", user, host, err)
						return
					}
					args := strings.Fields(cmdToRun)
					output, err := agent.RunCommandAndCaptureOutput(args[0], args[1:])
					if err != nil {
						results <- fmt.Sprintf("[%s@%s] command error: %v", user, host, err)
					} else {
						results <- fmt.Sprintf("[%s@%s] success:\n%s", user, host, output)
					}
				}(user, host)
			}
		}

		go func() {
			wg.Wait()
			close(results)
		}()

		for r := range results {
			fmt.Println(r)
		}

		duration := time.Since(start)
		fmt.Printf("\nCompleted SSH command across %d host(s) in %.2f seconds\n", totalHosts, duration.Seconds())

		return nil
	},
}

func init() {
	rootCmd.AddCommand(clusterSsh)

	clusterSsh.Flags().StringVar(&cmdToRun, "cmd", "", "Command to run on all remote hosts (required)")
	clusterSsh.Flags().StringToStringVar(&hostConnMap, "hostnames", nil, "Map of username to hostnames (e.g. --hostnames root=host1,host2 --hostnames jsmith=host3)")
	clusterSsh.Flags().StringVar(&configFilePath, "config-file", "", "Path to YAML config file containing hostnames map")
}

// ─────────────────────────────────────────────
// Step 1: Load from config file
func loadHostMapFromConfig(path string, output map[string][]string) error {
	if path == "" {
		return nil
	}
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	configHosts := v.GetStringMapStringSlice("hostnames")
	for user, hosts := range configHosts {
		output[user] = append(output[user], hosts...)
	}
	return nil
}

// ─────────────────────────────────────────────
// Step 2: Merge flags
func mergeHostMapFromFlags(flagMap map[string]string, output map[string][]string) {
	parsed := parseHostConnMap(flagMap)
	for user, hosts := range parsed {
		output[user] = append(output[user], hosts...)
	}
}

// ─────────────────────────────────────────────
// Step 3: Deduplicate
func dedupeHostMap(m map[string][]string) {
	for user, hosts := range m {
		m[user] = dedupe(hosts)
	}
}

func parseHostConnMap(in map[string]string) map[string][]string {
	out := make(map[string][]string)
	for user, hostsCSV := range in {
		hosts := strings.Split(hostsCSV, ",")
		for i := range hosts {
			hosts[i] = strings.TrimSpace(hosts[i])
		}
		out[user] = hosts
	}
	return out
}

func dedupe(in []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, val := range in {
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}
		if !seen[val] {
			seen[val] = true
			out = append(out, val)
		}
	}
	return out
}

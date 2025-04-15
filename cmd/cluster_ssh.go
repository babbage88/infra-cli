package cmd

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/babbage88/infra-cli/ssh"
	"github.com/spf13/cobra"
)

var (
	cmdToRun    string
	hostConnMap = make(map[string]string) // username -> comma-separated host list
)

var clusterSsh = &cobra.Command{
	Use:   "cluster-ssh",
	Short: "Execute SSH commands concurrently across multiple hosts",
	RunE: func(cmd *cobra.Command, args []string) error {
		start := time.Now()

		// Parse the --hostnames map[string]string into a usable map[string][]string
		hostMap := parseHostConnMap(hostConnMap)
		if len(hostMap) == 0 {
			return fmt.Errorf("no hosts provided")
		}

		if cmdToRun == "" {
			return fmt.Errorf("--cmd is required")
		}

		var wg sync.WaitGroup
		results := make(chan string, 50)
		totalHosts := 0

		for user, hosts := range hostMap {
			for _, host := range hosts {
				totalHosts++
				wg.Add(1)
				go func(user, host string) {
					defer wg.Done()
					agent, err := ssh.NewRemoteAppDeploymentAgentWithSshKey(
						host,
						user,
						"", "",
						rootViperCfg.GetString("ssh_key"),
						rootViperCfg.GetString("ssh_passphrase"),
						nil,                                   // no env vars
						rootViperCfg.GetBool("ssh_use_agent"), // use agent
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
						results <- fmt.Sprintf("[%s@%s] success running command. results\n\t\t%s\n\r", user, host, output)
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
		fmt.Printf("\n✅ Completed SSH command across %d host(s) in %.2f seconds\n", totalHosts, duration.Seconds())

		return nil
	},
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

func init() {
	rootCmd.AddCommand(clusterSsh)
	clusterSsh.Flags().StringVar(&cmdToRun, "cmd", "", "Command to run on all remote hosts (required)")
	clusterSsh.Flags().StringToStringVar(&hostConnMap, "hostnames", nil, "Map of username to hostnames (e.g. --hostnames root=host1,host2 --hostnames jsmith=host3)")
}

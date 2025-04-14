package cmd

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/babbage88/infra-cli/ssh"
)

var (
	cmdToRun    string
	hostConnMap = make(map[string]string) // username -> comma-separated host list
)

var clusterSsh = &cobra.Command{
	Use:   "cluster-ssh",
	Short: "Execute SSH commands concurrently across multiple hosts",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		for user, hosts := range hostMap {
			for _, host := range hosts {
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
					err = agent.RunCommandAndGetOutput(args[0], args[1:])
					if err != nil {
						results <- fmt.Sprintf("[%s@%s] command error: %v", user, host, err)
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

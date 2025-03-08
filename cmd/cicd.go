package cmd

import "github.com/spf13/cobra"

var cicdCmd = &cobra.Command{
	Use:   "cicd",
	Short: "CI/CD utilities and commands",
	Long: `Commands and utilities for build and deployment tasks fir this and other related modules.

bumper: utilitu to geneate the next iterates a given semver version based on the release type 
lgtm-guard: static analysis to prevent tokens being push to public repositories.

basically anything called right before, during, or right after a build or deployment pipeline.
`,
}

func init() {
	rootCmd.AddCommand(cicdCmd)
}

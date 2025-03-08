package cmd

import (
	"github.com/babbage88/infra-cli/internal/bumper"
	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/spf13/cobra"
)

var bumberCmd = &cobra.Command{
	Use:   "bumper",
	Short: "Generate the next version number/tag",
	Long: `Utility to geneate the next iterates a given semver version based on the release type 
lgtm-guard: static analysis to prevent tokens being push to public repositories.

Currently being called from a Makefile target named "release"
`,
	Run: func(cmd *cobra.Command, args []string) {
		_, err := bumper.BumpVersion(currentTag, incrementType)
		if err != nil {
			pretty.PrettyErrorLogF("Error generated the next version number. Latest Tag: %s error: %s", currentTag, err.Error())
		}
	},
}

var (
	currentTag    string
	incrementType string
)

func init() {
	cicdCmd.AddCommand(bumberCmd)
	// Default values from config or CLI flags
	generateCmd.Flags().StringVarP(&currentTag, "latest-version", "l", "", "The version number to analyze.")
	generateCmd.Flags().StringVarP(&incrementType, "increment-type", "i", "patch", "What type of release eg: major, minor, patch")

}

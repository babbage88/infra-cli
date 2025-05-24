package cmd

import (
	"github.com/babbage88/infra-cli/internal/bumper"
	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/spf13/cobra"
)

var bumperCmd = &cobra.Command{
	Use:   "bumper",
	Short: "Generate the next version number/tag",
	Long: `Utility to generate the next semver version based on the release type.

Currently being called from a Makefile target named "release".
`,
	Run: func(cmd *cobra.Command, args []string) {
		currentTag, _ := cmd.Flags().GetString("latest-version")
		incrementType, _ := cmd.Flags().GetString("increment-type")

		_, err := bumper.BumpVersion(currentTag, incrementType)
		if err != nil {
			pretty.PrettyErrorLogF("Error generating the next version number. Latest Tag: %s error: %s", currentTag, err.Error())
		}
	},
}

func init() {
	cicdCmd.AddCommand(bumperCmd)

	// Add flags to the bumperCmd, not global variables
	bumperCmd.Flags().StringP("latest-version", "l", "", "The version number to analyze.")
	bumperCmd.Flags().StringP("increment-type", "i", "patch", "What type of release eg: major, minor, patch")
}

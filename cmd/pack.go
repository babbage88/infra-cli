package cmd

import (
	"fmt"

	"github.com/babbage88/infra-cli/internal/files"
	"github.com/spf13/cobra"
)

var metaPackCmd = &cobra.Command{
	Use:   "pack",
	Short: "Create a tar.gz archive of a meta directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		srcDir, _ := cmd.Flags().GetString("src")
		outFile, _ := cmd.Flags().GetString("out")
		excludeList, _ := cmd.Flags().GetStringSlice("exclude")

		if srcDir == "" || outFile == "" {
			return fmt.Errorf("--src and --out are required")
		}

		fmt.Printf("Packing %s into %s\n", srcDir, outFile)
		if err := files.CreateTarGzWithExcludes(srcDir, outFile, excludeList); err != nil {
			return fmt.Errorf("failed to create archive: %w", err)
		}

		fmt.Println("✅ Archive created successfully.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(metaPackCmd)
	metaPackCmd.Flags().String("src", "", "Source directory to archive")
	metaPackCmd.Flags().String("out", "", "Output .tar.gz file path")
	metaPackCmd.Flags().StringSlice("exclude", []string{}, "Paths to exclude from the archive")

	metaCmd.AddCommand(metaPackCmd)
}

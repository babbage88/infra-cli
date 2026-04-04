package cmd

import "github.com/spf13/cobra"

var storageS3Cmd = &cobra.Command{
	Use:   "s3",
	Short: "Manage S3-compatible storage services",
}

func init() {
	storageCmd.AddCommand(storageS3Cmd)
}

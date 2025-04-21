package cmd

import (
	"github.com/spf13/cobra"
)

// batchCmd represents the base command for batch operations.
var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Manage OpenAI Batch API jobs",
	Long:  `Provides commands to interact with and view the status of OpenAI Batch API jobs used for embeddings.`,
	// Run: func(cmd *cobra.Command, args []string) { }, // Optional: Add default action or help
}

func init() {
	rootCmd.AddCommand(batchCmd)
	// Add subcommands like 'list', 'status', 'cancel' here in their respective files' init()
}

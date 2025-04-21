package cmd

import (
	"github.com/spf13/cobra"
)

// categorizeCmd represents the base command for categorization operations.
var categorizeCmd = &cobra.Command{
	Use:   "categorize",
	Short: "Manage content categorization (tags and collections)",
	Long: `Provides commands to automatically suggest and apply categories (tags and collections)
to content items using the configured categorization provider (e.g., LLM).`,
	// Run: func(cmd *cobra.Command, args []string) { }, // Optional: Add default action or help
}

func init() {
	rootCmd.AddCommand(categorizeCmd)
	// Add subcommands like 'apply', 'batch' here in their respective files' init()
}

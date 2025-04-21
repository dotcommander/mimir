package cmd

import (
	"fmt"
	"log"
	"strconv"

	"github.com/spf13/cobra"
	// "mimir/internal/app" // Removed unused import
	// "mimir/internal/config" // Removed unused import
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete [content_id]",
	Short: "Delete content and its associated embeddings",
	Long: `Deletes a specific content item identified by its ID.
This command attempts to remove associated vector embeddings first,
then removes the content record from the primary database.`,
	Args: cobra.ExactArgs(1), // Requires exactly one argument: the content ID
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse the content ID argument
		contentIDStr := args[0]
		contentID, err := strconv.ParseInt(contentIDStr, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid content ID provided: '%s'. Please provide a number.", contentIDStr)
		}

		log.Printf("Attempting to delete content with ID: %d", contentID)

		// Retrieve the application instance from context
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err // Error already formatted by GetAppFromContext
		}

		// --- Use ContentService directly ---
		if appInstance.ContentService == nil {
			return fmt.Errorf("content service is not initialized in the application")
		}
		// Pass VectorStore to allow attempting embedding deletion
		err = appInstance.ContentService.DeleteContent(cmd.Context(), contentID, appInstance.VectorStore)
		if err != nil {
			return fmt.Errorf("failed to delete content ID %d: %w", contentID, err)
		}

		fmt.Printf("Successfully deleted content with ID: %d\n", contentID)
		return nil // Return nil on success
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	// No flags needed for this basic version
}

package cmd

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	"mimir/internal/models" // Import models for Tag
	"mimir/internal/store"  // Import store for ErrNotFound

	"github.com/spf13/cobra"
)

// categorizeApplyCmd represents the apply command
var categorizeApplyCmd = &cobra.Command{
	Use:   "apply <content_id>",
	Short: "Suggest and automatically apply categories (tags/collection) to content",
	Long: `Fetches categorization suggestions (tags and a collection/category) for the specified
content ID using the configured provider and immediately applies them.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err
		}

		if appInstance.CategorizationService == nil {
			return fmt.Errorf("categorization service is not initialized or enabled")
		}

		contentIDStr := args[0]
		contentID, err := strconv.ParseInt(contentIDStr, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid content ID: %w", err)
		}

		log.Printf("Fetching content %d for categorization...", contentID)
		content, err := appInstance.ContentService.GetContent(cmd.Context(), contentID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return fmt.Errorf("content with ID %d not found", contentID)
			}
			return fmt.Errorf("failed to fetch content %d: %w", contentID, err)
		}

		log.Printf("Fetching existing tags for content %d...", contentID)
		existingTags, err := appInstance.TagService.GetContentTags(cmd.Context(), contentID)
		if err != nil {
			log.Printf("WARN: Failed to get existing tags for content %d: %v. Proceeding without them.", contentID, err)
			existingTags = []*models.Tag{} // Proceed with empty slice
		}
		existingTagNames := make([]string, len(existingTags))
		for i, tag := range existingTags {
			existingTagNames[i] = tag.Name
		}

		log.Printf("Requesting categorization suggestions for content %d...", contentID)
		suggestions, err := appInstance.CategorizationService.CategorizeContent(cmd.Context(), content.Title, content.Body, existingTagNames)
		if err != nil {
			return fmt.Errorf("failed to get categorization suggestions for content %d: %w", contentID, err)
		}

		fmt.Printf("Suggestions for Content ID %d:\n", contentID)
		fmt.Printf("  Suggested Tags: %v\n", suggestions.Tags)
		fmt.Printf("  Suggested Category: %s\n", suggestions.Category)
		fmt.Printf("  Confidence: %.2f\n", suggestions.Confidence)

		log.Printf("Applying suggestions for content %d...", contentID)
		err = appInstance.CategorizationService.ApplyCategories(cmd.Context(), contentID, suggestions, true) // autoApply = true
		if err != nil {
			// Log the error from ApplyCategories but don't necessarily fail the command
			// as some parts might have succeeded.
			log.Printf("ERROR applying categories for content %d: %v", contentID, err)
			fmt.Println("Warning: Errors occurred during category application. Check logs.")
			// Optionally return the error if full success is required:
			// return fmt.Errorf("failed to apply categories for content %d: %w", contentID, err)
		} else {
			fmt.Println("Successfully applied suggestions.")
		}

		return nil
	},
}

func init() {
	categorizeCmd.AddCommand(categorizeApplyCmd)
	// Add flags if needed (e.g., --dry-run, --force)
}

package cmd

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// tagCmd represents the tag command
var tagCmd = &cobra.Command{
	Use:   "tag [content_id] [tag_name...]",
	Short: "Associate tags with a content item",
	Long: `Adds one or more tags to a specific content item identified by its ID.
Tags are case-insensitive and will be created if they don't exist.`,
	Args: cobra.MinimumNArgs(2), // Requires content_id and at least one tag_name
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse the content ID argument
		contentIDStr := args[0]
		contentID, err := strconv.ParseInt(contentIDStr, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid content ID provided: '%s'. Please provide a number.", contentIDStr)
		}

		// Get the tag names (all remaining arguments)
		tagNames := args[1:]
		// Clean up tag names (e.g., trim whitespace) - optional but good practice
		cleanedTagNames := make([]string, 0, len(tagNames))
		for _, t := range tagNames {
			trimmed := strings.TrimSpace(t)
			if trimmed != "" {
				cleanedTagNames = append(cleanedTagNames, trimmed)
			}
		}

		if len(cleanedTagNames) == 0 {
			return fmt.Errorf("no valid tag names provided")
		}

		log.Printf("Attempting to tag content ID %d with tags: %v", contentID, cleanedTagNames)

		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err
		}

		if appInstance.TagService == nil {
			return fmt.Errorf("tag service is not initialized in the application")
		}

		_, err = appInstance.TagService.TagContent(cmd.Context(), contentID, cleanedTagNames)
		if err != nil {
			return fmt.Errorf("failed to apply tags to content ID %d: %w", contentID, err)
		}

		fmt.Printf("Successfully applied tags %v to content ID %d\n", cleanedTagNames, contentID)
		return nil // Return nil on success
	},
}

func init() {
	rootCmd.AddCommand(tagCmd)
	// No flags needed for this basic version
}

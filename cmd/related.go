package cmd

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	// "mimir/internal/app" // Removed unused import
	// "mimir/internal/config" // Removed unused import
	"mimir/internal/services"
)

var (
	relatedLimit int    // Variable to hold the limit flag value
	relatedTags  string // Flag for filtering related items by tags
)

var relatedCmd = &cobra.Command{
	Use:   "related <content_id>",
	Short: "Find content semantically similar to an existing item",
	Long: `Finds content items that are semantically similar to the specified content item,
based on vector embeddings. Requires the source content item to have been embedded.
Results are ordered by similarity score.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		contentIDStr := args[0]
		sourceContentID, err := strconv.ParseInt(contentIDStr, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid source content ID provided: '%s'. Please provide a number.", contentIDStr)
		}

		var filterTags []string
		if relatedTags != "" {
			filterTags = strings.Split(relatedTags, ",")
			for i := range filterTags {
				filterTags[i] = strings.TrimSpace(filterTags[i])
			}
			var cleaned []string
			for _, t := range filterTags {
				if t != "" {
					cleaned = append(cleaned, t)
				}
			}
			filterTags = cleaned
		}

		log.Printf("Finding content related to ID: %d (limit: %d, tags: %v)", sourceContentID, relatedLimit, filterTags)

		// Retrieve the application instance from context
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err // Error already formatted by GetAppFromContext
		}

		params := services.RelatedContentParams{
			SourceContentID: sourceContentID,
			Limit:           relatedLimit,
			FilterTags:      filterTags,
		}

		results, err := appInstance.SearchService.FindRelatedContent(cmd.Context(), params)
		if err != nil {
			log.Printf("Error finding related content: %v", err)
			return fmt.Errorf("failed to find content related to ID %d: %w", sourceContentID, err)
		}

		if len(results) == 0 {
			fmt.Printf("No related content found for ID %d (matching filters).\n", sourceContentID)
			return nil
		}

		fmt.Printf("Content Related to ID %d:\n", sourceContentID)
		fmt.Println("---------------------------")
		for _, item := range results {
			fmt.Printf("Score: %.4f\nID:    %d\nTitle: %s\n", item.Score, item.Content.ID, item.Content.Title)

			snippet := item.Content.Body
			maxLen := 200
			if len(snippet) > maxLen {
				lastSpace := strings.LastIndex(snippet[:maxLen], " ")
				if lastSpace > 0 {
					snippet = snippet[:lastSpace] + "..."
				} else {
					snippet = snippet[:maxLen] + "..."
				}
			}
			snippet = strings.ReplaceAll(snippet, "\n", " ")
			fmt.Printf("Snippet: %s\n---\n", snippet)
		}
		fmt.Println("---------------------------")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(relatedCmd)

	// Add flags
	relatedCmd.Flags().IntVarP(&relatedLimit, "limit", "n", 10, "Limit the number of related items to find")
	relatedCmd.Flags().StringVarP(&relatedTags, "tags", "T", "", "Comma-separated list of tags to filter related items by (match any)")
}

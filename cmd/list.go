package cmd

import (
	"fmt"
	"log"
	"strings" // For snippet processing

	"github.com/spf13/cobra"
	"mimir/internal/services"
	"mimir/internal/clix"
)

var (
	listLimit     int
	listOffset    int
	listSortBy    string
	listSortOrder string
	listTags      string // New flag for tags
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List stored content items",
	Long: `Displays a list of content items stored in the primary database.
Supports pagination, sorting, and filtering by tags.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Use centralized helpers for pagination and tags
		pagination, err := clix.ParsePagination(cmd.Flags())
		if err != nil {
			return err
		}
		filterTags, err := clix.ParseTags(cmd.Flags())
		if err != nil {
			return err
		}

		log.Printf("Executing list command: limit=%d, offset=%d, sortBy=%s, sortOrder=%s, tags=%v",
			pagination.Limit, pagination.Offset, listSortBy, listSortOrder, filterTags)

		// Get the initialized app instance from context
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to get app from context: %w", err)
		}

		// --- Use ContentService directly ---
		if appInstance.ContentService == nil {
			return fmt.Errorf("content service is not initialized in the application")
		}

		params := services.ListContentParams{
			Limit:      pagination.Limit,
			Offset:     pagination.Offset,
			SortBy:     listSortBy,
			SortOrder:  listSortOrder,
			FilterTags: filterTags,
		}
		results, err := appInstance.ContentService.ListContent(cmd.Context(), params)
		if err != nil {
			return fmt.Errorf("failed to list content: %w", err)
		}

		// Display results
		if len(results) == 0 {
			fmt.Println("No content found.")
			return nil
		}

		fmt.Println("Stored Content:")
		fmt.Println("---------------")
		for _, item := range results {
			fmt.Printf("ID: %d\nTitle: %s\nCreated: %s\n",
				item.Content.ID, item.Content.Title, item.Content.CreatedAt.Format("2006-01-02 15:04:05"))

			// Always display ModifiedAt, even if nil (show "N/A")
			if item.Content.ModifiedAt != nil {
				fmt.Printf("Modified: %s\n", item.Content.ModifiedAt.Format("2006-01-02 15:04:05"))
			} else {
				fmt.Printf("Modified: N/A\n")
			}

			// Display Tags
			if len(item.Tags) > 0 {
				tagNames := make([]string, len(item.Tags))
				for i, tag := range item.Tags {
					tagNames[i] = tag.Name
				}
				fmt.Printf("Tags: %s\n", strings.Join(tagNames, ", "))
			}

			// Display summary if present
			if item.Content.Summary != nil && *item.Content.Summary != "" {
				fmt.Printf("Summary: %s\n", *item.Content.Summary)
			}

			// Optionally display a snippet
			snippet := item.Content.Body
			maxSnippetLength := 100 // Shorter snippet for list view
			if len(snippet) > maxSnippetLength {
				snippet = snippet[:maxSnippetLength] + "..."
			}
			snippet = strings.ReplaceAll(snippet, "\n", " ")
			fmt.Printf("Snippet: %s\n---\n", snippet)
		}
		fmt.Println("---------------")
		fmt.Printf("Displayed %d items.\n", len(results)) // Use results here

		return nil // Return nil on success
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	// Add flags for pagination and sorting
	listCmd.Flags().IntVarP(&listLimit, "limit", "l", 20, "Number of items to display per page")
	listCmd.Flags().IntVarP(&listOffset, "offset", "o", 0, "Number of items to skip (for pagination)")
	listCmd.Flags().StringVar(&listSortBy, "sort-by", "c.created_at", "Column to sort by (c.id, c.title, c.created_at, c.updated_at)") // Prefix with 'c.'
	listCmd.Flags().StringVar(&listSortOrder, "sort-order", "desc", "Sort order (asc, desc)")
	listCmd.Flags().StringVarP(&listTags, "tags", "T", "", "Comma-separated list of tags to filter by (match any)")
}

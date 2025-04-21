package cmd

import (
	"fmt" // Added for printing results and errors
	"log"
	"strings" // Added for joining args

	"github.com/spf13/cobra"
	// "mimir/internal/app" // Removed unused import
	// "mimir/internal/config" // Removed unused import
	"mimir/internal/services" // Add services import
)

var (
	keywordTags string // New flag for tags
)

var keywordCmd = &cobra.Command{
	Use:   "keyword [query...]",
	Short: "Perform a keyword-based search (FTS), optionally filtered by tags",
	Long: `Performs a keyword-based search using the primary database's Full-Text Search (FTS) capabilities.
Provide the search terms as arguments.`,
	Args: cobra.MinimumNArgs(1), // Require at least one argument for the query
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ") // Join all arguments into a single query string

		// Parse tags flag
		var filterTags []string
		if keywordTags != "" {
			filterTags = strings.Split(keywordTags, ",")
			for i := range filterTags {
				filterTags[i] = strings.TrimSpace(filterTags[i])
			}
		}

		log.Printf("Performing keyword search for: '%s', tags: %v", query, filterTags)

		// Retrieve the application instance from context
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err // Error already formatted by GetAppFromContext
		}

		// --- Use SearchService directly ---
		if appInstance.SearchService == nil {
			return fmt.Errorf("search service is not initialized in the application")
		}

		params := services.KeywordSearchParams{
			Query:      query,
			FilterTags: filterTags,
			Limit:      0, // Limit/Offset not implemented in command flags yet
			Offset:     0,
		}
		results, err := appInstance.SearchService.KeywordSearch(cmd.Context(), params)
		if err != nil {
			return fmt.Errorf("keyword search failed: %w", err)
		}

		// Display results
		if len(results) == 0 {
			fmt.Println("No results found.")
			return nil
		}

		fmt.Println("Keyword Search Results:")
		fmt.Println("-----------------------")
		for _, item := range results {
			if item.Content == nil {
				log.Println("WARN: Skipping nil content in keyword search results")
				continue
			}
			fmt.Printf("ID: %d\nTitle: %s\n", item.Content.ID, item.Content.Title)

			// Print a snippet of the body
			snippet := item.Content.Body
			maxSnippetLength := 200 // Adjust as needed
			if len(snippet) > maxSnippetLength {
				snippet = snippet[:maxSnippetLength] + "..."
			}
			// Replace newlines in snippet for cleaner single-line display
			snippet = strings.ReplaceAll(snippet, "\n", " ")
			fmt.Printf("Snippet: %s\n---\n", snippet)
		}
		fmt.Println("-----------------------")

		return nil // Return nil on success
	},
}

func init() {
	rootCmd.AddCommand(keywordCmd)
	// Add flags
	keywordCmd.Flags().StringVarP(&keywordTags, "tags", "T", "", "Comma-separated list of tags to filter by (match any)")
	// keywordCmd.Flags().IntP("limit", "l", 50, "Limit the number of search results") // Limit is currently hardcoded in store
}

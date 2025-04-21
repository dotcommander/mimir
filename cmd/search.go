package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/cobra"
	"mimir/internal/services"
	"mimir/internal/clix"
)

var (
	searchLimit   int
	searchTags    string
	searchKeyword bool
)

var searchCmd = &cobra.Command{
	Use:   "search [query...]",
	Short: "Search content using semantic embeddings or keyword search",
	Long: `Performs a semantic search using vector embeddings by default.
Use --keyword to perform a traditional keyword-based search.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")

		pagination, err := clix.ParsePagination(cmd.Flags())
		if err != nil {
			return err
		}
		filterTags, err := clix.ParseTags(cmd.Flags())
		if err != nil {
			return err
		}

		log.Printf("Starting search (keyword=%v) for: '%s' (limit: %d, tags: %v)", searchKeyword, query, pagination.Limit, filterTags)

		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to get app from context: %w", err)
		}

		if appInstance.SearchService == nil {
			return fmt.Errorf("search service is not initialized in the application")
		}

		if searchKeyword {
			params := services.KeywordSearchParams{
				Query:      query,
				FilterTags: filterTags,
				Limit:      pagination.Limit,
				Offset:     0,
			}
			results, err := appInstance.SearchService.KeywordSearch(cmd.Context(), params)
			if err != nil {
				log.Printf("Error during keyword search: %v", err)
				return fmt.Errorf("keyword search failed: %w", err)
			}

			if len(results) == 0 {
				fmt.Println("No results found.")
				return nil
			}

			fmt.Println("Keyword Search Results:")
			fmt.Println("------------------------")
			for _, item := range results {
				if item.Content == nil {
					continue
				}
				fmt.Printf("Score: %.4f\nID:    %d\nTitle: %s\n", item.Score, item.Content.ID, item.Content.Title)
				// Always display ModifiedAt, even if nil (show "N/A")
				if item.Content.ModifiedAt != nil {
					fmt.Printf("Modified: %s\n", item.Content.ModifiedAt.Format("2006-01-02 15:04:05"))
				} else {
					fmt.Printf("Modified: N/A\n")
				}
				// Display summary if present
				if item.Content.Summary != nil && *item.Content.Summary != "" {
					fmt.Printf("Summary: %s\n", *item.Content.Summary)
				}
				snippet := item.Content.Body
				maxSnippetLength := 200
				if len(snippet) > maxSnippetLength {
					snippet = snippet[:maxSnippetLength] + "..."
				}
				snippet = strings.ReplaceAll(snippet, "\n", " ")
				fmt.Printf("Snippet: %s\n---\n", snippet)
			}
			fmt.Println("------------------------")
			return nil
		}

		params := services.SemanticSearchParams{
			Query:      query,
			Limit:      pagination.Limit,
			FilterTags: filterTags,
		}
		results, err := appInstance.SearchService.SemanticSearch(cmd.Context(), params)
		if err != nil {
			log.Printf("Error during semantic search: %v", err)
			return fmt.Errorf("semantic search failed: %w", err)
		}

		if len(results) == 0 {
			fmt.Println("No results found.")
			return nil
		}

		fmt.Println("Semantic Search Results:")
		fmt.Println("------------------------")
		for _, item := range results {
			if item.Content == nil {
				continue
			}
			fmt.Printf("Score: %.4f\nID:    %d\nTitle: %s\n", item.Score, item.Content.ID, item.Content.Title)
			// Always display ModifiedAt, even if nil (show "N/A")
			if item.Content.ModifiedAt != nil {
				fmt.Printf("Modified: %s\n", item.Content.ModifiedAt.Format("2006-01-02 15:04:05"))
			} else {
				fmt.Printf("Modified: N/A\n")
			}
			// Display summary if present
			if item.Content.Summary != nil && *item.Content.Summary != "" {
				fmt.Printf("Summary: %s\n", *item.Content.Summary)
			}

			// ChunkText and ChunkMetadata are no longer directly available on SearchResultItem
			// Display a snippet from the body as a fallback
			if item.Content.Body != "" {
				snippet := item.Content.Body
				maxSnippetLength := 200
				if len(snippet) > maxSnippetLength {
					snippet = snippet[:maxSnippetLength] + "..."
				}
				snippet = strings.ReplaceAll(snippet, "\n", " ")
				fmt.Printf("Snippet: %s\n", snippet)
			} else {
				fmt.Println("Snippet: (Body is empty)")
			}
		}
		fmt.Println("------------------------")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)

	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 10, "Limit the number of search results")
	searchCmd.Flags().StringVarP(&searchTags, "tags", "T", "", "Comma-separated list of tags to filter results by (match any)")
	searchCmd.Flags().BoolVar(&searchKeyword, "keyword", false, "Use keyword-based search instead of semantic search")
}

package cmd

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// categorizeBatchCmd represents the batch command
var categorizeBatchCmd = &cobra.Command{
	Use:   "batch <content_id1> [content_id2...]",
	Short: "Suggest categories (tags/collection) for multiple content items",
	Long: `Fetches categorization suggestions (tags and a collection/category) for the specified
content IDs using the configured provider. Does NOT automatically apply them.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err
		}

		if appInstance.CategorizationService == nil {
			return fmt.Errorf("categorization service is not initialized or enabled")
		}

		contentIDs := make([]int64, 0, len(args))
		for _, idStr := range args {
			contentID, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				log.Printf("WARN: Skipping invalid content ID '%s': %v", idStr, err)
				continue
			}
			contentIDs = append(contentIDs, contentID)
		}

		if len(contentIDs) == 0 {
			return fmt.Errorf("no valid content IDs provided")
		}

		log.Printf("Requesting batch categorization suggestions for %d content items...", len(contentIDs))
		resultsMap, err := appInstance.CategorizationService.BatchCategorize(cmd.Context(), contentIDs)
		if err != nil {
			return fmt.Errorf("failed to get batch categorization suggestions: %w", err)
		}

		fmt.Println("Batch Categorization Suggestions:")

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Content ID", "Suggested Tags", "Suggested Category", "Confidence"})
		table.SetBorder(true)
		table.SetRowLine(true)

		// Iterate through the original IDs to maintain order and show missing results
		for _, id := range contentIDs {
			idStr := strconv.FormatInt(id, 10)
			if cats, ok := resultsMap[id]; ok {
				table.Append([]string{
					idStr,
					fmt.Sprintf("%v", cats.Tags), // Simple formatting
					cats.Category,
					fmt.Sprintf("%.2f", cats.Confidence),
				})
			} else {
				// Indicate if categorization failed or was skipped for this ID
				table.Append([]string{idStr, "N/A", "N/A", "N/A"})
				log.Printf("WARN: No categorization result found for content ID %d (may have failed or been skipped)", id)
			}
		}

		table.Render()

		return nil
	},
}

func init() {
	categorizeCmd.AddCommand(categorizeBatchCmd)
	// Add flags if needed
}

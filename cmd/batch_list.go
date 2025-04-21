package cmd

import (
	"database/sql" // Add sql import for helper functions
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	// "mimir/internal/models" // Removed unused import

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	batchListLimit  int
	batchListOffset int
)

// batchListCmd represents the list command for batches
var batchListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recorded OpenAI Batch API jobs",
	Long:  `Lists the Batch API jobs that have been recorded in the Mimir database, showing their status and details.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err
		}

		if appInstance.BatchService == nil {
			return fmt.Errorf("batch service is not initialized")
		}

		log.Printf("Listing batch jobs (limit: %d, offset: %d)", batchListLimit, batchListOffset)

		batches, err := appInstance.BatchService.ListBatches(cmd.Context(), batchListLimit, batchListOffset)
		if err != nil {
			return fmt.Errorf("failed to list batch jobs: %w", err)
		}

		if len(batches) == 0 {
			fmt.Println("No batch jobs found.")
			return nil
		}

		// Display results in a table
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Batch ID", "Status", "Task Type", "Queue", "Created At", "Updated At", "Mimir Job ID"})
		table.SetBorder(true)  // Set true to draw borders
		table.SetRowLine(true) // Enable row line

		for _, batch := range batches {
			batchID := "N/A"
			if batch.BatchAPIJobID != nil {
				batchID = *batch.BatchAPIJobID
			}

			createdAtStr := batch.CreatedAt.Format(time.RFC3339)
			updatedAtStr := batch.UpdatedAt.Format(time.RFC3339)

			row := []string{
				batchID,
				batch.Status,
				batch.TaskType,
				batch.Queue,
				createdAtStr,
				updatedAtStr,
				batch.JobID.String(), // Mimir's internal job UUID
			}
			table.Append(row)
		}

		table.Render() // Send output

		return nil
	},
}

func init() {
	// Add batchListCmd as a subcommand of batchCmd (defined in cmd/batch.go)
	batchCmd.AddCommand(batchListCmd)

	// Add flags for pagination
	batchListCmd.Flags().IntVarP(&batchListLimit, "limit", "n", 20, "Maximum number of batch jobs to list")
	batchListCmd.Flags().IntVarP(&batchListOffset, "offset", "o", 0, "Number of batch jobs to skip")
}

// Helper function to safely get string from pointer or return default
func getStringPtrValue(ptr *string, def string) string {
	if ptr != nil {
		return *ptr
	}
	return def
}

// Helper function to safely get int64 from pointer or return default
func getInt64PtrValue(ptr *int64, def int64) int64 {
	if ptr != nil {
		return *ptr
	}
	return def
}

// Helper function to format nullable time
func formatNullTime(t *time.Time, layout string) string {
	if t != nil {
		return t.Format(layout)
	}
	return "N/A"
}

// Helper function to format nullable int64
func formatNullInt64(i *sql.NullInt64) string {
	if i != nil && i.Valid {
		return strconv.FormatInt(i.Int64, 10)
	}
	return "N/A"
}

// Helper function to format nullable string
func formatNullString(s *sql.NullString) string {
	if s != nil && s.Valid {
		return s.String
	}
	return "N/A"
}

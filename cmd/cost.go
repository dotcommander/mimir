package cmd

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter" // For aligned output

	"github.com/spf13/cobra"
	"mimir/internal/clix" // Use clix for pagination
)

var (
	costListLimit  int
	costListOffset int
)

// costCmd represents the base command for cost operations.
var costCmd = &cobra.Command{
	Use:   "cost",
	Short: "Manage and view AI usage costs",
	Long:  `Provides subcommands to list detailed AI usage logs and view cost summaries.`,
	// PersistentPreRunE can be added here if specific setup is needed for all cost subcommands
}

// costListCmd represents the command to list cost logs.
var costListCmd = &cobra.Command{
	Use:   "list",
	Short: "List detailed AI usage logs",
	Long:  `Displays a paginated list of recorded AI API calls with associated costs and token counts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err
		}

		if appInstance.CostService == nil {
			return fmt.Errorf("cost service is not initialized")
		}

		// Use clix helper for pagination flags
		pagination, err := clix.ParsePagination(cmd.Flags())
		if err != nil {
			return fmt.Errorf("invalid pagination flags: %w", err)
		}

		logs, err := appInstance.CostService.ListUsage(cmd.Context(), pagination.Limit, pagination.Offset)
		if err != nil {
			return fmt.Errorf("failed to list cost logs: %w", err)
		}

		if len(logs) == 0 {
			fmt.Println("No cost logs found.")
			return nil
		}

		// Use tabwriter for formatted output
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTimestamp\tProvider\tService\tModel\tIn Tokens\tOut Tokens\tCost\tContentID\tJobID")
		fmt.Fprintln(w, "--\t---------\t--------\t-------\t-----\t---------\t----------\t----\t---------\t-----")

		for _, log := range logs {
			contentIDStr := "N/A"
			if log.RelatedContentID != nil {
				contentIDStr = strconv.FormatInt(*log.RelatedContentID, 10)
			}
			jobIDStr := "N/A"
			if log.RelatedJobID != nil {
				jobIDStr = log.RelatedJobID.String()
			}

			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%d\t%d\t%.8f\t%s\t%s\n",
				log.ID,
				log.Timestamp.Format("2006-01-02 15:04:05"),
				log.ProviderName,
				log.ServiceType,
				log.ModelName,
				log.InputTokens,
				log.OutputTokens,
				log.Cost,
				contentIDStr,
				jobIDStr,
			)
		}
		w.Flush() // Flush the writer to ensure output is displayed

		fmt.Printf("\nDisplayed %d logs.\n", len(logs))
		return nil
	},
}

// costSummaryCmd represents the command to view cost summary.
var costSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show summary of total AI costs and token usage",
	Long:  `Calculates and displays the total cost, total input tokens, and total output tokens across all recorded AI usage.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err
		}

		if appInstance.CostService == nil {
			return fmt.Errorf("cost service is not initialized")
		}

		totalCost, totalInput, totalOutput, err := appInstance.CostService.GetSummary(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to get cost summary: %w", err)
		}

		fmt.Println("AI Usage Cost Summary:")
		fmt.Println("----------------------")
		fmt.Printf("Total Cost:        $%.6f\n", totalCost)
		fmt.Printf("Total Input Tokens: %d\n", totalInput)
		fmt.Printf("Total Output Tokens:%d\n", totalOutput)
		fmt.Println("----------------------")

		return nil
	},
}

func init() {
	// Add subcommands to the base cost command
	costCmd.AddCommand(costListCmd)
	costCmd.AddCommand(costSummaryCmd)

	// Add flags for the list subcommand using the clix variables
	costListCmd.Flags().IntVarP(&costListLimit, "limit", "l", 50, "Number of logs to display")
	costListCmd.Flags().IntVarP(&costListOffset, "offset", "o", 0, "Number of logs to skip")

	// Add the base cost command to the root command (this happens in root.go's init)
	// rootCmd.AddCommand(costCmd) // This line is moved to root.go
}

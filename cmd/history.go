package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	historyLimit int
)

// historyCmd represents the base command for search history operations
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View search history",
	Long:  `Displays past search queries recorded by the application.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default to list subcommand's help or run list directly?
		// Let's run list directly for simplicity.
		listHistoryCmd.Run(cmd, args)
	},
}

var listHistoryCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent search queries",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err
		}

		queries, err := appInstance.SearchService.ListSearchHistory(cmd.Context(), historyLimit)
		if err != nil {
			return fmt.Errorf("error listing search history: %w", err)
		}

		if len(queries) == 0 {
			fmt.Println("No search history found.")
			return nil
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"ID", "Query", "Results", "Executed At"})
		table.SetBorder(false)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)

		for _, q := range queries {
			table.Append([]string{
				strconv.FormatInt(q.ID, 10),
				q.Query,
				strconv.Itoa(q.ResultsCount),
				q.ExecutedAt.Format("2006-01-02 15:04:05"),
			})
		}
		table.Render()
		return nil
	},
}

func init() {
	// Add historyCmd to root command in root.go's init

	// Flags for list command
	listHistoryCmd.Flags().IntVarP(&historyLimit, "limit", "n", 20, "Maximum number of history entries to show")

	// Add subcommands to historyCmd
	historyCmd.AddCommand(listHistoryCmd)

	// Add historyCmd to the root command
	rootCmd.AddCommand(historyCmd)
}

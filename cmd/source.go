package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	sourceName        string
	sourceDescription string
	sourceURL         string
	sourceType        string
)

// sourceCmd represents the base command for source management
var sourceCmd = &cobra.Command{
	Use:   "source",
	Short: "Manage content sources",
	Long:  `List and create sources from which content originates.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// createSourceCmd represents the command to create a new source
var createSourceCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new source",
	Args:  cobra.NoArgs, // Use flags
	RunE: func(cmd *cobra.Command, args []string) error {
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err
		}

		if appInstance.SourceService == nil {
			return fmt.Errorf("source service is not initialized")
		}

		source, err := appInstance.SourceService.CreateSource(cmd.Context(), sourceName, sourceType, &sourceDescription, &sourceURL)
		if err != nil {
			return fmt.Errorf("error creating source: %w", err)
		}

		fmt.Printf("Successfully created source: ID=%d, Name='%s'\n", source.ID, source.Name)
		return nil
	},
}

// listSourcesCmd represents the command to list all sources
var listSourcesCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sources",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err
		}

		if appInstance.SourceService == nil {
			return fmt.Errorf("source service is not initialized")
		}

		sources, err := appInstance.SourceService.ListSources(cmd.Context())
		if err != nil {
			return fmt.Errorf("error listing sources: %w", err)
		}

		if len(sources) == 0 {
			fmt.Println("No sources found.")
			return nil
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"ID", "Name", "Type", "Description", "URL", "Created At"})
		table.SetBorder(false)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)

		for _, s := range sources {
			desc := ""
			if s.Description != nil {
				desc = *s.Description
			}
			url := ""
			if s.URL != nil {
				url = *s.URL
			}
			table.Append([]string{
				strconv.FormatInt(s.ID, 10),
				s.Name,
				s.SourceType,
				desc,
				url,
				s.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		table.Render()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(sourceCmd)

	// Flags for create source command
	createSourceCmd.Flags().StringVarP(&sourceName, "name", "n", "", "Name of the source (required)")
	createSourceCmd.Flags().StringVarP(&sourceType, "type", "t", "", "Type of the source (e.g., file, url, note) (required)")
	createSourceCmd.Flags().StringVarP(&sourceDescription, "description", "d", "", "Description of the source")
	createSourceCmd.Flags().StringVarP(&sourceURL, "url", "u", "", "URL associated with the source")
	createSourceCmd.MarkFlagRequired("name")
	createSourceCmd.MarkFlagRequired("type")

	// Add subcommands to sourceCmd
	sourceCmd.AddCommand(createSourceCmd)
	sourceCmd.AddCommand(listSourcesCmd)
}

package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"mimir/internal/fileingest"
	"mimir/internal/services"
)

// addRecursiveCmd represents the recursive add command.
var addRecursiveCmd = &cobra.Command{
	Use:   "add-recursive [directory]",
	Short: "Recursively add all markdown files in a directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := args[0]
		ctx := cmd.Context()
		appInstance, err := GetAppFromContext(ctx)
		if err != nil {
			return fmt.Errorf("failed to get app instance: %w", err)
		}
		files, err := fileingest.DiscoverMarkdownFiles(ctx, dir)
		if err != nil {
			return fmt.Errorf("failed to discover markdown files: %w", err)
		}
		if len(files) == 0 {
			fmt.Printf("No markdown files found under %s\n", dir)
			return nil
		}
		fmt.Printf("Discovered %d markdown files under %s:\n", len(files), dir)
		for _, f := range files {
			fmt.Printf("  - %s (size: %d bytes, modified: %s)\n", f.Path, f.Size, f.ModTime.Format("2006-01-02 15:04:05"))
		}
		fmt.Println("Ready to process: chunk, embed, summarize, tag, and store each file.")

		var successCount, errorCount int
		defer func() {
			fmt.Printf("\nProcessed %d files: %d succeeded, %d failed\n",
				len(files), successCount, errorCount)
		}()

		for _, f := range files {
			fmt.Printf("\nProcessing: %s\n", f.Path)

			content, existed, err := appInstance.ContentService.AddContent(ctx, services.AddContentParams{
				RawInput:   f.Path,
				SourceName: filepath.Base(dir),
				SourceType: "directory",
				Title:      strings.TrimSuffix(f.Name, ".md"),
			})

			if err != nil {
				errorCount++
				fmt.Printf("  - %s: %v\n", color.RedString("ERROR"), err)
				continue
			}

			status := color.GreenString("Processed")
			if existed {
				status = color.YellowString("Existed")
			}
			fmt.Printf("  - %s ID:%d\n", status, content.ID)
			successCount++
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addRecursiveCmd)
}

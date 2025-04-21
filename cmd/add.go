package cmd

import (
	"errors"
	"fmt"
	"io/fs" // Required for errors.Is(walkErr, fs.ErrPermission)
	"log"
	"net/url" // Add net/url import
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"mimir/internal/services"
)

var (
	addTitle  string
	addSource string
	// addInput is removed as we use positional arg now
)

var addCmd = &cobra.Command{
	Use:   "add [input]", // Expect one positional argument
	Short: "Add new content to Mimir",
	Long: `Adds new content from a file path, URL, or raw text string provided as an argument.
If --title is not provided, it defaults to the base name of the input file path.
If --source is not provided, it defaults to 'local'.
The input will be processed, stored, and an embedding job will be queued.`,
	Args: cobra.ExactArgs(1), // Exactly one positional argument is required
	RunE: func(cmd *cobra.Command, args []string) error {
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err
		}

		if appInstance.ContentService == nil {
			return fmt.Errorf("content service is not initialized in the application")
		}

		// Get input from the positional argument
		rawInput := args[0]

		// Determine absolute path for consistent handling and directory checking
		absInput, err := filepath.Abs(rawInput)
		if err != nil {
			// If we can't get absolute path, log warning but proceed cautiously.
			// Directory check might fail if rawInput is relative and invalid.
			log.Printf("WARN: Could not determine absolute path for '%s': %v. Proceeding with original input.", rawInput, err)
			absInput = rawInput // Fallback, but directory check below might be less reliable
		}

		// Check if input is a directory
		stat, statErr := os.Stat(absInput) // Use absInput for reliable check

		// --- Directory Mode ---
		if statErr == nil && stat.IsDir() {
			fmt.Printf("Processing directory: %s\n", absInput)
			var filesProcessed, filesAdded, filesSkipped, filesErrored int

			// Determine source name for directory add
			// Use --source if provided, otherwise default to the directory's base name
			dirSource := addSource
			if dirSource == "" || dirSource == "local" { // If default or explicitly local, use dir name
				dirSource = filepath.Base(absInput)
				log.Printf("Using directory name '%s' as source (override with --source).", dirSource)
			} else {
				log.Printf("Using provided source name '%s' for directory add.", dirSource)
			}

			walkErr := filepath.WalkDir(absInput, func(path string, d os.DirEntry, walkErr error) error {
				if walkErr != nil {
					// Error accessing path (e.g., permissions)
					fmt.Printf("  - ERROR accessing %s: %v\n", path, walkErr)
					filesErrored++
					// Allow WalkDir to continue with other files/subdirs unless error is severe
					if errors.Is(walkErr, fs.ErrPermission) {
						// If it's a directory permission error, skip the whole directory
						if d != nil && d.IsDir() {
							return filepath.SkipDir
						}
					}
					return nil // Continue walking
				}

				// Skip the root directory itself
				if path == absInput {
					return nil
				}

				// Skip hidden files and directories (e.g., .git, .DS_Store)
				if strings.HasPrefix(d.Name(), ".") {
					if d.IsDir() {
						log.Printf("Skipping hidden directory: %s", path)
						return filepath.SkipDir // Skip processing this directory further
					}
					log.Printf("Skipping hidden file: %s", path)
					return nil // Skip this hidden file
				}

				// Skip subdirectories, only process files
				if d.IsDir() {
					return nil // Continue into subdirectories
				}

				// Process only .md files (case-insensitive)
				if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
					return nil // Skip non-markdown files
				}

				// --- Process the .md file ---
				filesProcessed++
				// Use filename without extension as title
				title := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
				// Use the determined directory source name
				params := services.AddContentParams{
					SourceName: dirSource,
					Title:      title,
					RawInput:   path, // Use the full, absolute path to the file
					SourceType: "cli-directory",
				}

				log.Printf("Adding file: Title='%s', Source='%s', Input='%s'", params.Title, params.SourceName, params.RawInput)
				content, existed, addErr := appInstance.ContentService.AddContent(cmd.Context(), params)

				if addErr != nil {
					fmt.Printf("  - ERROR adding %s: %v\n", path, addErr)
					filesErrored++
				} else if existed {
					fmt.Printf("  - Skipped (exists): %s (ID: %d)\n", path, content.ID)
					filesSkipped++
				} else {
					fmt.Printf("  - Added: %s (ID: %d)\n", path, content.ID)
					filesAdded++
				}
				return nil // Continue with the next file
			}) // End WalkDir

			if walkErr != nil {
				// This error is from WalkDir setup itself, not the callback function
				fmt.Printf("Error walking directory %s: %v\n", absInput, walkErr)
				// Return the error to indicate the command failed overall
				return fmt.Errorf("directory walk failed: %w", walkErr)
			}

			fmt.Println("------------------------------------")
			fmt.Printf("Directory processing complete.\n")
			fmt.Printf("Files Found (.md): %d\n", filesProcessed)
			fmt.Printf("Files Added:       %d\n", filesAdded)
			fmt.Printf("Files Skipped:     %d\n", filesSkipped)
			fmt.Printf("Errors:            %d\n", filesErrored)
			fmt.Println("------------------------------------")
			return nil // Success for directory mode

		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			// Stat failed with an error other than "does not exist" (e.g., permission denied on the input itself)
			return fmt.Errorf("failed to stat input '%s': %w", absInput, statErr)
		}

		// --- Single Item Mode (File, URL, or Raw Text) ---
		// If it wasn't a directory or stat failed with ErrNotExist, treat as single item.

		// Set default source if not provided
		source := addSource
		if source == "" {
			source = "local" // Keep default as "local" for single items
		}

		// Set default title from filename if not provided and input looks like a file path
		title := addTitle
		if title == "" {
			// Only default title if it wasn't explicitly set AND it's not clearly a URL
			_, urlErr := url.ParseRequestURI(rawInput)
			// Check if it's NOT a URL AND it doesn't contain common characters suggesting raw text (like spaces)
			// This is heuristic, might need refinement.
			if urlErr != nil && !strings.ContainsAny(rawInput, " \n\t") {
				base := filepath.Base(rawInput)
				title = strings.TrimSuffix(base, filepath.Ext(base))
				// Handle cases where TrimSuffix leaves an empty string (e.g., ".bashrc")
				if title == "" && base != "" {
					title = base
				}
				log.Printf("Defaulting title to '%s' based on input.", title)
			}
			// If it was a URL or raw text, title might remain empty here, which is acceptable.
			// ContentService might apply further defaults if needed.
		}

		params := services.AddContentParams{
			SourceName: source,
			Title:      title,
			RawInput:   rawInput, // Use the original input here for the processor
			SourceType: "cli",    // Indicate it came directly from CLI arg
		}

		log.Printf("Adding single item: Title='%s', Source='%s', Input='%s'", params.Title, params.SourceName, params.RawInput)

		content, existed, err := appInstance.ContentService.AddContent(cmd.Context(), params)
		if err != nil {
			return fmt.Errorf("failed to add content: %w", err)
		}

		if existed {
			fmt.Printf("Content already exists (ID: %d). Skipped.\n", content.ID)
		} else {
			fmt.Printf("Content added (ID: %d). Embedding and other jobs enqueued.\n", content.ID)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().StringVarP(&addTitle, "title", "t", "", "Optional title (defaults to input filename)")
	addCmd.Flags().StringVarP(&addSource, "source", "s", "local", "Optional source name (defaults to 'local')")
	// Remove the --input flag as it's now a positional argument
	// Remove MarkFlagRequired calls
}

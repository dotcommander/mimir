package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	collectionName        string
	collectionDescription string
	collectionIsPinned    bool
	collectionID          int64
	contentID             int64
	// Re-use list flags from list.go (or define locally if preferred)
	// listLimit     int // Defined in list.go
	// listOffset    int // Defined in list.go
	// listSortBy    string // Defined in list.go
	// listSortOrder string // Defined in list.go
)

// collectionCmd represents the base command when called without any subcommands
var collectionCmd = &cobra.Command{
	Use:   "collection",
	Short: "Manage content collections",
	Long:  `Create, list, and manage collections of content items.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var createCollectionCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new collection",
	Args:  cobra.NoArgs, // Expect flags instead of positional args
	RunE: func(cmd *cobra.Command, args []string) error {
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err
		}

		collection, err := appInstance.CollectionService.CreateCollection(cmd.Context(), collectionName, &collectionDescription, collectionIsPinned)
		if err != nil {
			return fmt.Errorf("failed to create collection: %w", err)
		}
		fmt.Printf("Successfully created collection: ID=%d, Name='%s'\n", collection.ID, collection.Name)
		return nil
	},
}

var listCollectionsCmd = &cobra.Command{
	Use:   "list",
	Short: "List all collections",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err
		}

		collections, err := appInstance.CollectionService.ListCollections(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to list collections: %w", err)
		}

		if len(collections) == 0 {
			fmt.Println("No collections found.")
			return nil
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"ID", "Name", "Description", "Pinned", "Created At"})
		table.SetBorder(false)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)

		for _, c := range collections {
			desc := ""
			if c.Description != nil {
				desc = *c.Description
			}
			pinned := "No"
			if c.IsPinned {
				pinned = "Yes"
			}
			table.Append([]string{
				strconv.FormatInt(c.ID, 10),
				c.Name,
				desc,
				pinned,
				c.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		table.Render()
		return nil
	},
}

var addContentToCollectionCmd = &cobra.Command{
	Use:   "add",
	Short: "Add content to a collection",
	Args:  cobra.NoArgs, // Use flags
	RunE: func(cmd *cobra.Command, args []string) error {
		if contentID <= 0 || collectionID <= 0 {
			return fmt.Errorf("both --content-id and --collection-id flags must be provided and positive")
		}

		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err
		}

		err = appInstance.CollectionService.AddContent(cmd.Context(), contentID, collectionID)
		if err != nil {
			return fmt.Errorf("failed adding content %d to collection %d: %w", contentID, collectionID, err)
		}
		fmt.Printf("Successfully added content %d to collection %d (or association already existed).\n", contentID, collectionID)
		return nil
	},
}

var removeContentFromCollectionCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove content from a collection",
	Args:  cobra.NoArgs, // Use flags
	RunE: func(cmd *cobra.Command, args []string) error {
		if contentID <= 0 || collectionID <= 0 {
			return fmt.Errorf("both --content-id and --collection-id flags must be provided and positive")
		}

		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err
		}

		err = appInstance.CollectionService.RemoveContent(cmd.Context(), contentID, collectionID)
		if err != nil {
			return fmt.Errorf("failed removing content %d from collection %d: %w", contentID, collectionID, err)
		}
		fmt.Printf("Successfully removed content %d from collection %d (if association existed).\n", contentID, collectionID)
		return nil
	},
}

var listContentByCollectionCmd = &cobra.Command{
	Use:   "list-content",
	Short: "List content within a specific collection",
	Args:  cobra.NoArgs, // Use flags
	RunE: func(cmd *cobra.Command, args []string) error {
		if collectionID <= 0 {
			return fmt.Errorf("--collection-id flag must be provided and positive")
		}

		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err
		}

		// Validate sort order
		sortOrderUpper := strings.ToUpper(listSortOrder)
		if sortOrderUpper != "ASC" && sortOrderUpper != "DESC" {
			fmt.Fprintf(os.Stderr, "Warning: Invalid sort order '%s'. Defaulting to DESC.\n", listSortOrder)
			sortOrderUpper = "DESC"
		}
		// Validate sort by - prefix with 'c.' for DB query if needed
		sortByInternal := listSortBy
		switch listSortBy {
		case "id", "title", "created_at", "updated_at":
			sortByInternal = "c." + listSortBy // Assuming 'c' is the alias for contents table in the store query
		default:
			fmt.Fprintf(os.Stderr, "Warning: Invalid sort by column '%s'. Defaulting to created_at.\n", listSortBy)
			sortByInternal = "c.created_at"
		}

		results, err := appInstance.CollectionService.ListContent(cmd.Context(), collectionID, listLimit, listOffset, sortByInternal, sortOrderUpper)
		if err != nil {
			return fmt.Errorf("failed listing content for collection %d: %w", collectionID, err)
		}

		if len(results) == 0 {
			fmt.Printf("No content found in collection %d.\n", collectionID)
			return nil
		}

		// Reuse the table rendering from listCmd
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"ID", "Title", "Type", "Tags", "Created At"}) // Add more fields if needed
		table.SetBorder(false)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetRowLine(true) // Add lines between rows for readability

		for _, item := range results {
			tagNames := make([]string, len(item.Tags))
			for i, tag := range item.Tags {
				tagNames[i] = tag.Name
			}
			tagsStr := strings.Join(tagNames, ", ")
			table.Append([]string{
				strconv.FormatInt(item.Content.ID, 10),
				item.Content.Title,
				item.Content.ContentType,
				tagsStr, // Display tags fetched by the service
				item.Content.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		table.Render()
		return nil
	},
}

func init() {
	// Add collectionCmd to the root command
	// This is done in cmd/root.go's init() or main() usually,
	// but we need to ensure the subcommands are added first.

	// Flags for create command
	createCollectionCmd.Flags().StringVarP(&collectionName, "name", "n", "", "Name of the collection (required)")
	createCollectionCmd.Flags().StringVarP(&collectionDescription, "description", "d", "", "Description of the collection")
	createCollectionCmd.Flags().BoolVar(&collectionIsPinned, "pinned", false, "Pin the collection")
	createCollectionCmd.MarkFlagRequired("name") // Make name mandatory

	// Flags for add/remove commands
	addContentToCollectionCmd.Flags().Int64VarP(&contentID, "content-id", "c", 0, "ID of the content item (required)")
	addContentToCollectionCmd.Flags().Int64VarP(&collectionID, "collection-id", "l", 0, "ID of the collection (required)")
	// addContentToCollectionCmd.MarkFlagRequired("content-id") // Mark required below
	// addContentToCollectionCmd.MarkFlagRequired("collection-id")

	removeContentFromCollectionCmd.Flags().Int64VarP(&contentID, "content-id", "c", 0, "ID of the content item (required)")
	removeContentFromCollectionCmd.Flags().Int64VarP(&collectionID, "collection-id", "l", 0, "ID of the collection (required)")
	// removeContentFromCollectionCmd.MarkFlagRequired("content-id") // Mark required below
	// removeContentFromCollectionCmd.MarkFlagRequired("collection-id")

	// Flags for list-content command
	listContentByCollectionCmd.Flags().Int64VarP(&collectionID, "collection-id", "l", 0, "ID of the collection (required)")
	// Use persistent flags from listCmd for pagination/sorting
	listContentByCollectionCmd.Flags().IntVar(&listLimit, "limit", 20, "Maximum number of items to list")
	listContentByCollectionCmd.Flags().IntVar(&listOffset, "offset", 0, "Number of items to skip")
	listContentByCollectionCmd.Flags().StringVar(&listSortBy, "sort-by", "created_at", "Column to sort by (id, title, created_at, updated_at)")
	listContentByCollectionCmd.Flags().StringVar(&listSortOrder, "sort-order", "desc", "Sort order (asc, desc)")
	// listContentByCollectionCmd.MarkFlagRequired("collection-id") // Mark required below

	// Add subcommands to collectionCmd
	collectionCmd.AddCommand(createCollectionCmd)
	collectionCmd.AddCommand(listCollectionsCmd)
	collectionCmd.AddCommand(addContentToCollectionCmd)
	collectionCmd.AddCommand(removeContentFromCollectionCmd)
	collectionCmd.AddCommand(listContentByCollectionCmd)

	// Mark flags required *after* adding subcommands if they are shared or specific
	addContentToCollectionCmd.MarkFlagRequired("content-id")
	addContentToCollectionCmd.MarkFlagRequired("collection-id")
	removeContentFromCollectionCmd.MarkFlagRequired("content-id")
	removeContentFromCollectionCmd.MarkFlagRequired("collection-id")
	listContentByCollectionCmd.MarkFlagRequired("collection-id")

	// Add the main collection command to the root command
	rootCmd.AddCommand(collectionCmd)
}

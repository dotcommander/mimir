package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"mimir/internal/app"
	"mimir/internal/config"
	"mimir/internal/inputprocessor" // Import the package
)

var rootCmd = &cobra.Command{
	Use:   "mimir",
	Short: "Mimir CLI App",
	Long:  `Mimir is a CLI tool for managing and searching your personal knowledge base using embeddings.`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is given, print help.
		cmd.Help()
	},
	// PersistentPreRunE runs before any subcommand's RunE
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Don't run initialization for help command or potentially others
		if cmd.Name() == "help" || cmd.Name() == "version" { // Add other commands to skip if needed
			return nil
		}

		// Load configuration once
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize input processor
		// Use the new constructor for the concrete implementation
		inputProc := inputprocessor.New() // Corrected constructor name

		// Initialize the app once, passing the input processor
		appInstance, err := app.NewApp(cfg, inputProc)
		if err != nil {
			return fmt.Errorf("failed to initialize app: %w", err)
		}

		// Store the app instance in the command's context
		ctx := context.WithValue(cmd.Context(), appKey, appInstance)
		cmd.SetContext(ctx) // Update the command's context
		return nil
	},
}

// init function registers flags for root command if needed.
// Subcommands (like add, search, list, tag, collection, etc.) are added
// in their respective init() functions (e.g., cmd/add.go, cmd/collection.go)
// which are called automatically by Go's initialization process before main() runs.

// func init() {
// Allow flags to be interspersed for all commands
// rootCmd.PersistentFlags().SetInterspersed(true) // Removed global setting
// }

func Execute() {
	// Initialization or setup can happen here before Execute() is called by main()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Define a custom type for the context key to avoid collisions.
type contextKey string

const appKey contextKey = "app"

// Helper function to retrieve the app instance from context
func GetAppFromContext(ctx context.Context) (*app.App, error) {
	appInstance, ok := ctx.Value(appKey).(*app.App)
	if !ok || appInstance == nil {
		// This should not happen if PersistentPreRunE ran successfully
		return nil, fmt.Errorf("application instance not found in context")
	}
	return appInstance, nil
}

func init() {
	// Initialization flags for rootCmd can go here if needed

	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(costCmd) // Add the cost command
	// batchCmd is added in cmd/batch.go's init()
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check database connectivity and other diagnostics",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		appInstance, err := GetAppFromContext(ctx)
		if err != nil {
			return fmt.Errorf("failed to get app instance: %w", err)
		}

		fmt.Println("Checking database connectivity...")

		if err := appInstance.ContentStore.Ping(ctx); err != nil {
			return fmt.Errorf("database ping failed: %w", err)
		}

		fmt.Println("Database connection successful.")
		return nil
	},
}

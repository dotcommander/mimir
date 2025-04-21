package cmd

import (
	"fmt"
	"log"
	"mimir/internal/apihandlers" // Import the new handlers package
	// "mimir/internal/app" // Removed unused import
	// "mimir/internal/config" // Removed unused import
	"net/http" // Required for http constants

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

var (
	serveAddr string // Listen address
	servePort string // Listen port
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run Mimir as an HTTP API server",
	Long: `Starts an HTTP server exposing Mimir functionalities (add, search, list)
via a RESTful API. Allows interaction from other tools or UIs.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Retrieve the application instance from context
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return err // Error already formatted by GetAppFromContext
		}

		// Setup Gin router
		// gin.SetMode(gin.ReleaseMode) // Uncomment for production
		router := gin.Default() // Includes logger and recovery middleware

		// --- Setup API Routes ---
		apiHandler := apihandlers.NewAPIHandler(appInstance) // Create handler instance

		// Group routes under /api/v1 (optional, but good practice)
		v1 := router.Group("/api/v1")
		{
			// Content Routes
			contentGroup := v1.Group("/content")
			{
				contentGroup.POST("", apiHandler.AddContentHandler)
				contentGroup.GET("", apiHandler.ListContentHandler)
				contentGroup.GET("/:id", apiHandler.GetContentHandler)
				// TODO: Add PUT /content/:id for editing later?
				// TODO: Add DELETE /content/:id later?
			}

			// Search Routes (Semantic)
			searchGroup := v1.Group("/search")
			{
				searchGroup.GET("", apiHandler.SearchContentHandler) // Semantic search
			}
			// Keyword Search Routes
			keywordGroup := v1.Group("/keyword")
			{
				keywordGroup.GET("", apiHandler.KeywordSearchHandler) // Keyword search
			}

			// TODO: Add routes for tags, collections, related, history etc. later
			// Example:
			// tagGroup := v1.Group("/tags") { ... }
			// collectionGroup := v1.Group("/collections") { ... }
		}

		// Simple health check endpoint
		router.GET("/health", func(c *gin.Context) {
			// TODO: Add checks for DB/Redis connectivity if needed
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Start the server
		listenAddr := fmt.Sprintf("%s:%s", serveAddr, servePort)
		log.Printf("Starting Mimir API server on http://%s", listenAddr)

		// router.Run blocks unless an error occurs
		if err := router.Run(listenAddr); err != nil {
			// Log the error before returning it
			log.Printf("ERROR: Failed to run API server: %v", err)
			return fmt.Errorf("failed to run API server: %w", err)
		}

		// This part is unlikely to be reached if router.Run starts successfully
		log.Println("Mimir API server stopped.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Add flags for server configuration
	serveCmd.Flags().StringVar(&serveAddr, "addr", "localhost", "Address to listen on (e.g., '0.0.0.0' for all interfaces)")
	serveCmd.Flags().StringVar(&servePort, "port", "8080", "Port to listen on")
}

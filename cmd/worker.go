package cmd

import (
	"context"
	"fmt"    // Add fmt import
	"log"
	"os"        // For signal handling
	"os/signal" // For signal handling
	"syscall"   // For signal handling

	"github.com/hibiken/asynq"
	"github.com/spf13/cobra"
	"mimir/internal/app"
	"mimir/internal/tasks" // Add tasks import
	"mimir/internal/worker" // Add worker import
)

// workerCmd represents the worker command
var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Run the background job worker",
	Long:  `Starts the Asynq worker process to handle background tasks like embedding generation.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Retrieve the application instance from context
		appInstance, err := GetAppFromContext(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to get application context: %w", err)
		}

		// Run the worker logic using the initialized app instance
		if err := runWorker(appInstance); err != nil {
			// Log the error before exiting
			log.Printf("FATAL: Worker exited with error: %v", err)
			return err // Return the error to potentially set exit code
		}
		return nil // Success
	},
}

func init() {
	rootCmd.AddCommand(workerCmd)
	// Add flags specific to the worker if needed (e.g., concurrency, queues)
	// Add flags specific to the worker if needed
}

// runWorker initializes and runs the Asynq worker server.
func runWorker(appInstance *app.App) error {
	cfg := appInstance.Config // Use config from the initialized app

	// --- Setup Asynq Server ---
	redisOpts := asynq.RedisClientOpt{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}

	srv := asynq.NewServer(
		redisOpts,
		asynq.Config{
			Concurrency: cfg.Worker.Concurrency,
			Queues:      cfg.Worker.Queues,
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				log.Printf("ERROR: Asynq task failed: task_id=%s type=%s payload=%s err=%v",
					task.ResultWriter().TaskID(), task.Type(), string(task.Payload()), err)
				// Add more sophisticated error handling/reporting here
			}),
			// Logger: // Custom logger if needed
		},
	)

	// --- Register Job Handlers ---
	mux := asynq.NewServeMux()

	// Create EmbeddingDeps from appInstance and config
	embeddingDeps := worker.EmbeddingDeps{
		Fetcher:       appInstance.ContentStore,
		Generator:     appInstance.EmbeddingService,
		Storer:        appInstance.VectorStore,
		Updater:       appInstance.ContentStore,
		JobStore:      appInstance.JobStore,
		BatchProvider: appInstance.BatchAPIProvider,
		JobClient:     appInstance.JobClient,
		MaxTokens:     cfg.Chunking.MaxTokens, // Pass config values
		Overlap:       cfg.Chunking.Overlap,
		UseBatchAPI:   cfg.Embedding.UseBatchAPI,
	}
	// Register Embedding & Batch Check Handlers (using the new registration function)
	worker.RegisterHandlers(mux, embeddingDeps, cfg)

	// Register Summarization Handler (if service is available)
	if appInstance.SummaryService != nil {
		// Check if it's the Noop service if you want to avoid registering it
		// if _, ok := appInstance.SummaryService.(*services.NoopSummaryService); !ok {
		summarizationDeps := worker.SummarizationDeps{
			ContentStore:   appInstance.ContentStore,
			SummaryService: appInstance.SummaryService,
		}
		log.Printf("Registering SummarizationJob handler (%s)", tasks.TypeSummarizationJob)
		mux.HandleFunc(tasks.TypeSummarizationJob, worker.HandleSummarizationJob(summarizationDeps))
		// } else {
		// 	log.Println("INFO: SummaryService is Noop, skipping registration of summarization handler.")
		// }
	} else {
		log.Println("WARN: SummaryService is nil, skipping registration of summarization handler.")
	}

	// Register other handlers here...

	// --- Start Server & Handle Shutdown ---
	log.Printf("Starting Asynq worker server (Concurrency: %d, Queues: %v)...", cfg.Worker.Concurrency, cfg.Worker.Queues)
	if err := srv.Start(mux); err != nil {
		return fmt.Errorf("failed to start Asynq server: %w", err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown

	log.Println("Shutdown signal received. Initiating graceful shutdown...")
	// Graceful shutdown timeout is handled by the context passed to Shutdown() or Stop()
	// srv.ShutdownTimeout = 30 * time.Second // This field does not exist
	srv.Stop()
	srv.Shutdown()

	// Optional: Add cleanup for appInstance resources if needed
	// appInstance.Close()

	log.Println("Worker shutdown complete.")
	return nil
}

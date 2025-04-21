package tests

import (
	"context"
	"testing"

	"mimir/internal/app"
	"mimir/internal/config"         // Import config package
	"mimir/internal/inputprocessor" // Import inputprocessor package
)

func TestAppInitialization(t *testing.T) {
	// Load default config (or create a dummy one if preferred for isolation)
	// Note: This might require a config.yaml or env vars to be set depending on LoadConfig implementation
	cfg, err := config.LoadConfig()
	if err != nil {
		// If loading fails, create a minimal dummy config for the test
		t.Logf("Warning: Failed to load config, using minimal dummy config: %v", err)
		cfg = &config.Config{} // Provide a minimal non-nil config
		// Optionally populate dummy DSNs if NewApp requires them
		// cfg.Database.Primary.DSN = "dummy_dsn"
		// cfg.Database.Vector.DSN = "dummy_dsn"
		// cfg.Redis.Address = "dummy_redis:6379"
	}

	// Create a default input processor
	processor := inputprocessor.New()

	// Call NewApp with config and processor
	a, err := app.NewApp(cfg, processor)
	if err != nil {
		t.Fatalf("Failed to initialize App: %v", err)
	}
	ctx := context.Background()
	// Check that essential App components are non-nil.
	// Check individual store interfaces instead of PrimaryStore
	if a.ContentStore == nil {
		t.Error("ContentStore is nil")
	}
	if a.TagStore == nil {
		t.Error("TagStore is nil")
	}
	// Add checks for other essential stores/clients if needed
	// if a.SourceStore == nil {
	// 	t.Error("SourceStore is nil")
	// }
	// if a.CollectionStore == nil {
	// 	t.Error("CollectionStore is nil")
	// }
	// if a.SearchHistoryStore == nil {
	// 	t.Error("SearchHistoryStore is nil")
	// }
	if a.VectorStore == nil {
		t.Error("VectorStore is nil")
	}
	if a.EmbeddingService == nil {
		t.Error("EmbeddingService is nil")
	}
	if a.JobClient == nil {
		t.Error("JobClient is nil")
	}
	// As an example, enqueue an embedding job.
	// Note: This might fail if Redis isn't running during the test.
	// Consider mocking the JobClient or skipping this in unit tests.
	// Call EnqueueEmbeddingJob via the JobClient interface
	err = a.JobClient.EnqueueEmbeddingJob(ctx, 1) // Pass context and contentID
	if err != nil {
		// Log the error but don't fail the test just for enqueue failure in this basic check
		t.Logf("Warning: Failed to enqueue embedding job during app test (is Redis running?): %v", err)
	}
	// Optionally, additional integration tests can go here.
}

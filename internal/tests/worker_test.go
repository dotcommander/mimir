package tests

import (
	"testing"

	"github.com/hibiken/asynq"
	"mimir/internal/app"
	"mimir/internal/tasks" // Added for task type constant
	"mimir/internal/worker"
)

func TestWorkerRegistration(t *testing.T) {
	a, err := app.NewApp()
	if err != nil {
		t.Fatalf("Failed to initialize App: %v", err)
	}
	mux := asynq.NewServeMux()

	// Create the EmbeddingDeps struct from the app components
	deps := worker.EmbeddingDeps{
		Fetcher:   a.PrimaryStore, // PrimaryStore implements GetContent
		Generator: a.EmbeddingService,
		Storer:    a.VectorStore,  // VectorStore implements AddEmbedding
		Updater:   a.PrimaryStore, // PrimaryStore implements UpdateContentEmbeddingStatus
	}

	// Pass the dependencies struct to RegisterHandlers
	worker.RegisterHandlers(mux, deps, a.Config)

	// Check if the handler for the specific task type is registered
	// Use the constant from the tasks package
	handlerInfo := mux.HandlerInfo(tasks.TypeEmbeddingJob) // Use tasks.TypeEmbeddingJob
	if handlerInfo.Handler == nil {
		t.Errorf("Expected handler for task type '%s' to be registered, but it was nil", tasks.TypeEmbeddingJob)
	}
}

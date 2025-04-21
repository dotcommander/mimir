package services

import (
	"context"
	"sync"

	"github.com/pgvector/pgvector-go"
	"github.com/sashabaranov/go-openai" // Add import for openai.Batch type

	"mimir/internal/store" // Add missing store import
)

// ProviderStatus is now defined in internal/store/interfaces.go

type EmbeddingProvider interface {
	Name() string
	ModelName() string
	Status() store.ProviderStatus // Change return type to store.ProviderStatus
	GenerateEmbedding(ctx context.Context, text string) (pgvector.Vector, error)
	GenerateEmbeddings(ctx context.Context, texts []string) ([]pgvector.Vector, error)
	Dimension() int
}

type RetryStrategy interface {
	NextBackoff(attempt int) int64 // ms
}

type FallbackEmbeddingService struct {
	Providers      []EmbeddingProvider
	ActiveProvider int
	RetryStrategy  RetryStrategy
	mu             sync.RWMutex
}

// ModelName returns the model name of the currently active provider.
// Returns empty string if no providers or active provider cannot be determined.
func (s *FallbackEmbeddingService) ModelName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.Providers) == 0 || s.ActiveProvider < 0 || s.ActiveProvider >= len(s.Providers) {
		return "" // No active provider
	}
	return s.Providers[s.ActiveProvider].ModelName()
}

// Name returns the name of the currently active provider.
// Returns empty string if no providers or active provider cannot be determined.
func (s *FallbackEmbeddingService) Name() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.Providers) == 0 || s.ActiveProvider < 0 || s.ActiveProvider >= len(s.Providers) {
		return "" // No active provider
	}
	return s.Providers[s.ActiveProvider].Name()
}

// Status returns the status of the currently active provider.
func (s *FallbackEmbeddingService) Status() store.ProviderStatus { // Change return type to store.ProviderStatus
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.Providers) == 0 || s.ActiveProvider < 0 || s.ActiveProvider >= len(s.Providers) {
		return store.ProviderStatusDisabled // Use store constant
	}
	// Assuming s.Providers[x].Status() now returns store.ProviderStatus
	return s.Providers[s.ActiveProvider].Status()
}

// Ensure FallbackEmbeddingService implements the updated store.EmbeddingService interface
var _ store.EmbeddingService = (*FallbackEmbeddingService)(nil)

// --- Batch API Interface ---
// Defined here for broader accessibility

// BatchAPIProvider defines methods for interacting with a Batch API (like OpenAI's).
type BatchAPIProvider interface {
	CreateFile(ctx context.Context, fileName string, fileContent []byte) (string, error)
	CreateBatch(ctx context.Context, inputFileID, endpoint string, completionWindow string) (openai.BatchResponse, error) // Corrected return type
	RetrieveBatch(ctx context.Context, batchID string) (openai.BatchResponse, error)                                      // Corrected return type
	GetFileContent(ctx context.Context, fileID string) ([]byte, error)
	// Add other necessary methods like CancelBatch, ListBatches if needed
}

// SimpleRetryStrategy provides basic exponential backoff.
type SimpleRetryStrategy struct {
	MaxAttempts int
	BaseDelayMs int64
}

// NextBackoff calculates the next backoff duration in milliseconds.
func (s *SimpleRetryStrategy) NextBackoff(attempt int) int64 {
	if s.MaxAttempts <= 0 { // If MaxAttempts is 0 or negative, don't retry
		return -1
	}
	if attempt >= s.MaxAttempts {
		return -1 // Stop retrying
	}
	// Simple exponential backoff: BaseDelay * 2^attempt
	backoff := s.BaseDelayMs * (1 << attempt)
	// Add some jitter? Cap maximum delay? For now, keep it simple.
	// Cap at e.g. 30 seconds
	maxDelay := int64(30000)
	if backoff > maxDelay {
		backoff = maxDelay
	}
	return backoff
}

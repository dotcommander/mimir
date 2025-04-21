package services

import (
	"context"
	"errors" // Add errors import
	"fmt"
	"os"

	"mimir/internal/store" // ProviderStatus is defined here

	"github.com/google/generative-ai-go/genai"
	"github.com/pgvector/pgvector-go"
	log "github.com/sirupsen/logrus" // Or your preferred logger
	"google.golang.org/api/option"
)

// GeminiProvider implements both EmbeddingService and potentially CompletionService (partially) using the Google Gemini API.
type GeminiProvider struct {
	client          *genai.Client
	embeddingModel  string // Store the embedding model name, e.g., "models/embedding-001"
	completionModel string // Store the completion model name, e.g., "models/gemini-pro"
	dim             int    // Store the embedding dimension
	// Add cost tracking dependencies if needed for completion
}

// NewGeminiProvider creates a new Gemini embedding provider.
func NewGeminiProvider(apiKey, modelName string) (*GeminiProvider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY") // Fallback to env var
	}
	if apiKey == "" {
		log.Warn("Gemini API key not provided. Gemini provider will be disabled.")
		return &GeminiProvider{client: nil}, nil
		// Or return error: return nil, fmt.Errorf("Gemini API key not provided")
	}

	// Determine dimension based on model name (add more models as needed)
	var dim int
	switch modelName {
	case "models/embedding-001":
		dim = 768
	// Add other known Gemini embedding models and their dimensions
	// case "models/text-embedding-004": // Example future model
	// 	dim = 768
	default:
		log.Warnf("Unknown Gemini embedding model '%s', defaulting dimension to 768 (embedding-001). Accuracy may be affected.", modelName)
		dim = 768
		// Or return error: return nil, fmt.Errorf("unknown or unsupported Gemini embedding model: %s", modelName)
	}

	ctx := context.Background() // Use background context for initialization
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		// Don't return the provider if client creation fails
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	log.Infof("Gemini provider initialized with model %s (dimension %d)", modelName, dim)

	return &GeminiProvider{
		client:         client,
		embeddingModel: modelName,
		// completionModel: // Set this if/when completion is fully supported
		dim:    dim,
	}, nil
}

// Name returns the provider name.
func (p *GeminiProvider) Name() string { return "gemini" }

// ModelName returns the specific model identifier.
func (p *GeminiProvider) ModelName() string { return p.embeddingModel } // Return embedding model for EmbeddingService interface

// GenerateEmbedding generates an embedding for a single text.
func (p *GeminiProvider) GenerateEmbedding(ctx context.Context, text string) (pgvector.Vector, error) {
	if p.client == nil {
		return pgvector.Vector{}, fmt.Errorf("Gemini provider is not initialized (missing API key)")
	}
	if text == "" {
		log.Warn("GenerateEmbedding called with empty text for Gemini")
		vec := pgvector.NewVector(make([]float32, p.dim)) // Return zero vector
		// ensureVectorStatus(&vec)
		return vec, nil
	}

	em := p.client.EmbeddingModel(p.embeddingModel)
	// Call EmbedContent for single text
	res, err := em.EmbedContent(ctx, genai.Text(text))
	if err != nil {
		// TODO: Implement more robust error handling (check specific Gemini errors)
		return pgvector.Vector{}, fmt.Errorf("Gemini API error generating embedding: %w", err)
	}

	// Check if Embedding or Values are nil/empty
	if res == nil || res.Embedding == nil || len(res.Embedding.Values) == 0 {
		return pgvector.Vector{}, fmt.Errorf("Gemini API returned no embedding data")
	}

	// Ensure the returned dimension matches expected dimension
	if len(res.Embedding.Values) != p.dim {
		log.Warnf("Gemini returned embedding dimension %d, expected %d for model %s", len(res.Embedding.Values), p.dim, p.embeddingModel)
		return pgvector.Vector{}, fmt.Errorf("Gemini API returned unexpected embedding dimension: got %d, want %d", len(res.Embedding.Values), p.embeddingModel)
	}

	vec := pgvector.NewVector(res.Embedding.Values)
	// ensureVectorStatus(&vec)
	return vec, nil
}

// GenerateEmbeddings generates embeddings for multiple texts.
func (p *GeminiProvider) GenerateEmbeddings(ctx context.Context, texts []string) ([]pgvector.Vector, error) {
	if p.client == nil {
		return nil, fmt.Errorf("Gemini provider is not initialized (missing API key)")
	}
	if len(texts) == 0 {
		return []pgvector.Vector{}, nil
	}

	em := p.client.EmbeddingModel(p.embeddingModel)
	results := make([]pgvector.Vector, len(texts))

	for i, text := range texts {
		if text == "" {
			results[i] = pgvector.NewVector(make([]float32, p.dim)) // Zero vector for empty input
			// ensureVectorStatus(&results[i])
			continue
		}

		res, err := em.EmbedContent(ctx, genai.Text(text))
		if err != nil {
			// Handle error for individual embedding, maybe return partial results or fail all?
			// Failing all for now.
			return nil, fmt.Errorf("Gemini API error generating embedding for text at index %d: %w", i, err)
		}

		if res == nil || res.Embedding == nil || len(res.Embedding.Values) == 0 {
			return nil, fmt.Errorf("Gemini API returned no embedding data for text at index %d", i)
		}

		results[i] = pgvector.NewVector(res.Embedding.Values)
		// ensureVectorStatus(&results[i])
	}

	return results, nil
}

// GenerateChatCompletion implements the CompletionService interface (basic implementation).
// NOTE: This is a placeholder. A proper implementation would need model selection,
// prompt formatting, and potentially cost tracking specific to completion.
func (p *GeminiProvider) GenerateChatCompletion(ctx context.Context, messages []ChatMessage) (string, error) {
	if p.client == nil {
		return "", fmt.Errorf("Gemini provider is not initialized (missing API key)")
	}
	if p.completionModel == "" {
		return "", errors.New("Gemini provider is not configured for chat completion (completion model not set)")
	}
	// TODO: Implement actual chat completion logic using p.client.GenerativeModel(p.completionModel)
	return "", errors.New("Gemini chat completion not yet fully implemented in this provider")
}

// Dimension returns the expected embedding dimension for the configured model.
func (p *GeminiProvider) Dimension() int {
	return p.dim
}

// Status returns the operational status of the provider.
func (p *GeminiProvider) Status() store.ProviderStatus { // Use store.ProviderStatus
	if p.client == nil {
		return store.ProviderStatusDisabled // Use store constant
	}
	// TODO: Implement actual status check (e.g., simple API ping if available)
	// For now, assume OK if client is initialized.
	return store.ProviderStatusActive // Use store constant
}

// Close cleans up the Gemini client resources.
func (p *GeminiProvider) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

// Ensure GeminiProvider implements the interface at compile time.
var _ store.EmbeddingService = (*GeminiProvider)(nil)
// Ensure GeminiProvider implements CompletionService
var _ CompletionService = (*GeminiProvider)(nil) // Check CompletionService implementation

// Add Closer interface if needed for graceful shutdown
// var _ io.Closer = (*GeminiProvider)(nil)

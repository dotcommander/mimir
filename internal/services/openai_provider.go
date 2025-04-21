package services

import (
	"context"
	"fmt"
	"os"

	"time" // Add time import

	"mimir/internal/config" // Add config import
	"mimir/internal/models"
	"mimir/internal/store"

	"github.com/pgvector/pgvector-go"
	"github.com/sashabaranov/go-openai"

	log "github.com/sirupsen/logrus"
)

// OpenAIProvider implements the EmbeddingService using the OpenAI API.
type OpenAIProvider struct {
	client    *openai.Client
	model     openai.EmbeddingModel // Store the model ID
	dim       int                   // Store the dimension
	costStore store.CostTrackingStore
	pricing   map[string]config.PricingInfo

}

// NewOpenAIProvider creates a new OpenAI embedding provider.
func NewOpenAIProvider(apiKey, modelID string, costStore store.CostTrackingStore, pricing map[string]config.PricingInfo) (*OpenAIProvider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY") // Fallback to env var
	}
	if apiKey == "" {
		log.Warn("OpenAI API key not provided. OpenAI provider will be disabled.")
		return &OpenAIProvider{client: nil}, nil
	}

	var dim int
	switch modelID {
	case string(openai.AdaEmbeddingV2):
		dim = 1536
	case "text-embedding-3-small":
		dim = 1536
	case "text-embedding-3-large":
		dim = 3072
	default:
		log.Warnf("Unknown OpenAI embedding model '%s', defaulting dimension to 1536 (AdaV2). Accuracy may be affected.", modelID)
		dim = 1536
	}

	client := openai.NewClient(apiKey)
	log.Infof("OpenAI provider initialized with model %s (dimension %d)", modelID, dim)

	return &OpenAIProvider{
		client:    client,
		model:     openai.EmbeddingModel(modelID),
		dim:       dim,
		costStore: costStore,
		pricing:   pricing, // Assign the passed pricing map
	}, nil
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string { return "openai" }

// ModelName returns the specific model identifier.
func (p *OpenAIProvider) ModelName() string { return string(p.model) }

func (p *OpenAIProvider) GenerateEmbedding(ctx context.Context, text string) (pgvector.Vector, error) {
	if p.client == nil {
		return pgvector.Vector{}, fmt.Errorf("OpenAI provider is not initialized (missing API key)")
	}
	if text == "" {
		log.Warn("GenerateEmbedding called with empty text for OpenAI")
		return pgvector.NewVector(make([]float32, p.dim)), nil
	}

	req := openai.EmbeddingRequestStrings{
		Input: []string{text},
		Model: p.model,
	}

	resp, err := p.client.CreateEmbeddings(ctx, req)
	if err != nil {
		return pgvector.Vector{}, fmt.Errorf("OpenAI API error generating embedding: %w", err)
	}

	if len(resp.Data) == 0 || len(resp.Data[0].Embedding) == 0 {
		return pgvector.Vector{}, fmt.Errorf("OpenAI API returned no embedding data")
	}

	if len(resp.Data[0].Embedding) != p.dim {
		log.Warnf("OpenAI returned embedding dimension %d, expected %d for model %s", len(resp.Data[0].Embedding), p.dim, p.model)
		return pgvector.Vector{}, fmt.Errorf("OpenAI API returned unexpected embedding dimension: got %d, want %d", len(resp.Data[0].Embedding), p.dim)
	}

	// --- Cost Tracking Instrumentation ---
	if p.costStore != nil && resp.Usage.TotalTokens > 0 {
		priceInfo, ok := p.pricing[p.ModelName()] // Use local pricing map
		if !ok {
			log.Warnf("Pricing info not found for model '%s'. Cannot record cost.", string(p.model))
		} else {
			// Assuming embedding only uses input tokens for cost calculation based on OpenAI pricing model
			cost := float64(resp.Usage.TotalTokens) * priceInfo.InputPerToken
			logEntry := &models.AIUsageLog{
				Timestamp:    time.Now(), // Use current time as CreatedAt might not be available or accurate
				ProviderName: p.Name(),
				ServiceType:  "embedding",
				ModelName:    p.ModelName(),
				InputTokens:  resp.Usage.TotalTokens, // OpenAI embedding usage reports total tokens
				OutputTokens: 0,                      // No output tokens for embeddings
				Cost:         cost,
				// RelatedContentID and RelatedJobID should be set by the caller if available
			}
			if err := p.costStore.RecordUsage(ctx, logEntry); err != nil {
				log.Errorf("Failed to record AI usage log for embedding: %v", err)
				// Decide if this error should be propagated
			} else {
				log.Debugf("Recorded AI usage: Provider=%s, Service=%s, Model=%s, Tokens=%d, Cost=%.8f",
					logEntry.ProviderName, logEntry.ServiceType, logEntry.ModelName, logEntry.InputTokens, logEntry.Cost)
			}
		}
	}
	// --- End Cost Tracking ---

	vec := pgvector.NewVector(resp.Data[0].Embedding)
	return vec, nil
}

func (p *OpenAIProvider) GenerateEmbeddings(ctx context.Context, texts []string) ([]pgvector.Vector, error) {
	if p.client == nil {
		return nil, fmt.Errorf("OpenAI provider is not initialized (missing API key)")
	}
	if len(texts) == 0 {
		return []pgvector.Vector{}, nil
	}

	validTexts := make([]string, 0, len(texts))
	originalIndices := make(map[int]int)
	for i, t := range texts {
		if t != "" {
			originalIndices[len(validTexts)] = i
			validTexts = append(validTexts, t)
		} else {
			log.Warnf("GenerateEmbeddings called with empty text at index %d for OpenAI", i)
		}
	}

	if len(validTexts) == 0 {
		results := make([]pgvector.Vector, len(texts))
		for i := range results {
			results[i] = pgvector.NewVector(make([]float32, p.dim))
		}
		return results, nil
	}

	req := openai.EmbeddingRequestStrings{
		Input: validTexts,
		Model: p.model,
	}

	resp, err := p.client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error generating embeddings: %w", err)
	}

	if len(resp.Data) != len(validTexts) {
		return nil, fmt.Errorf("OpenAI API returned %d embeddings, expected %d", len(resp.Data), len(validTexts))
	}

	// --- Cost Tracking Instrumentation ---
	if p.costStore != nil && resp.Usage.TotalTokens > 0 {
		priceInfo, ok := p.pricing[p.ModelName()] // Use local pricing map
		if !ok {
			log.Warnf("Pricing info not found for model '%s'. Cannot record cost.", string(p.model))
		} else {
			// Assuming embedding only uses input tokens for cost calculation
			cost := float64(resp.Usage.TotalTokens) * priceInfo.InputPerToken
			logEntry := &models.AIUsageLog{
				Timestamp:    time.Now(),
				ProviderName: p.Name(),
				ServiceType:  "embedding",
				ModelName:    p.ModelName(),
				InputTokens:  resp.Usage.TotalTokens,
				OutputTokens: 0,
				Cost:         cost,
			}
			if err := p.costStore.RecordUsage(ctx, logEntry); err != nil {
				log.Errorf("Failed to record AI usage log for batch embedding: %v", err)
			} else {
				log.Debugf("Recorded AI usage: Provider=%s, Service=%s, Model=%s, Tokens=%d, Cost=%.8f",
					logEntry.ProviderName, logEntry.ServiceType, logEntry.ModelName, logEntry.InputTokens, logEntry.Cost)
			}
		}
	}
	// --- End Cost Tracking ---

	results := make([]pgvector.Vector, len(texts))
	for i := range results {
		results[i] = pgvector.NewVector(make([]float32, p.dim))
	}

	for i, data := range resp.Data {
		if len(data.Embedding) != p.dim {
			log.Warnf("OpenAI returned embedding dimension %d for index %d, expected %d for model %s", len(data.Embedding), i, p.dim, p.model)
			return nil, fmt.Errorf("OpenAI API returned unexpected embedding dimension in batch: got %d, want %d at index %d", len(data.Embedding), p.dim, i)
		}
		originalIndex := originalIndices[i]
		results[originalIndex] = pgvector.NewVector(data.Embedding)
	}

	return results, nil
}

// Dimension returns the expected embedding dimension for the configured model.
func (p *OpenAIProvider) Dimension() int {
	// Return the stored dimension
	return p.dim
}

// Status returns the operational status of the provider.
func (p *OpenAIProvider) Status() store.ProviderStatus { // Change return type to store.ProviderStatus
	if p.client == nil {
		return store.ProviderStatusDisabled // Use store constant
	}
	// TODO: Implement actual status check (e.g., simple API ping if available)
	// For now, assume OK if client is initialized.
	return store.ProviderStatusActive // Use store constant
}

var _ store.EmbeddingService = (*OpenAIProvider)(nil)

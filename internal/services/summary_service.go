package services

import (
	"context"
	"fmt"
	"mimir/internal/config" // Add config import
	"time"

	"mimir/internal/models"
	"mimir/internal/store"

	log "github.com/sirupsen/logrus" // Or your preferred logger
	"github.com/sashabaranov/go-openai"
	"github.com/google/uuid"
)

type SummaryService interface {
	// Add contentID and jobID parameters for cost tracking context
	Summarize(ctx context.Context, text string, contentID int64, jobID string) (string, error)
}

// OpenAISummaryService implements SummaryService using OpenAI.
type OpenAISummaryService struct {
	client    *openai.Client
	model     string
	prompt    string // Add prompt field
	costStore store.CostTrackingStore
	pricing   map[string]config.PricingInfo

}

// NewOpenAISummaryService creates a new summary service using OpenAI.
func NewOpenAISummaryService(apiKey, model, prompt string, costStore store.CostTrackingStore, pricing map[string]config.PricingInfo) *OpenAISummaryService {
	if apiKey == "" {
		log.Warn("OpenAI API key not provided for summarization. Service will be disabled.")
		return &OpenAISummaryService{client: nil} // Return disabled service
	}
	client := openai.NewClient(apiKey)
	return &OpenAISummaryService{
		client:    client,
		model:     model,
		prompt:    prompt,
		costStore: costStore,
		pricing:   pricing,
	}
}

// Add Model method for cost tracking
func (s *OpenAISummaryService) Model() string {
	return s.model
}

// Add contentID and jobID parameters
func (s *OpenAISummaryService) Summarize(ctx context.Context, text string, contentID int64, jobID string) (string, error) {
	if s.client == nil {
		return "", fmt.Errorf("OpenAISummaryService is not initialized (missing API key)")
	}

	resp, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: s.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    "system",
				Content: s.prompt, // Use configured prompt
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Summarize this in one paragraph:\n\n%s", text),
			},
		},
	})

	if err != nil {
		return "", fmt.Errorf("openai completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	// --- Cost Tracking Instrumentation ---
	if s.costStore != nil && resp.Usage.TotalTokens > 0 {
		priceInfo, ok := s.pricing[s.model]
		if !ok {
			log.Warnf("Pricing info not found for model '%s'. Cannot record cost for summarization.", s.model)
		} else {
			cost := float64(resp.Usage.PromptTokens)*priceInfo.InputPerToken +
				float64(resp.Usage.CompletionTokens)*priceInfo.OutputPerToken
			logEntry := &models.AIUsageLog{
				Timestamp:    time.Now(),
				ProviderName: "openai", // Assuming OpenAI
				ServiceType:  "summarization",
				ModelName:    s.model,
				InputTokens:  resp.Usage.PromptTokens,
				OutputTokens: resp.Usage.CompletionTokens,
				Cost:         cost,
				RelatedContentID: &contentID, // Set RelatedContentID
				// RelatedJobID: // Set below after parsing
			}
			// Attempt to parse jobID if it's a valid UUID string
			if jid, err := uuid.Parse(jobID); err == nil {
				logEntry.RelatedJobID = &jid // Store UUID if valid
			} else {
				log.Warnf("Could not parse jobID '%s' as UUID for cost tracking: %v", jobID, err)
			}

			if err := s.costStore.RecordUsage(ctx, logEntry); err != nil {
				log.Errorf("Failed to record AI usage log for summarization: %v", err)
			} else {
				log.Debugf("Recorded AI usage: Provider=%s, Service=%s, Model=%s, InputTokens=%d, OutputTokens=%d, Cost=%.8f",
					logEntry.ProviderName, logEntry.ServiceType, logEntry.ModelName, logEntry.InputTokens, logEntry.OutputTokens, logEntry.Cost)
			}
		}
	}
	// --- End Cost Tracking ---

	return resp.Choices[0].Message.Content, nil
}

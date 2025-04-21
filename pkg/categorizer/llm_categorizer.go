package categorizer

import (
	"context"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus" // Use logrus
	"strings"
	"time"
	"mimir/internal/config" // Import config package
	"mimir/internal/costtracker" // Import costtracker

	"github.com/sashabaranov/go-openai"
)

// LLMCategorizer implements ContentCategorizer
// relies on an LLM/completion API
type LLMCategorizer struct {
	client interface {
		CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
	}
	promptTemplate string
	model string

	// Dependencies for cost tracking
	costTracker costtracker.CostTracker       // Use CostTracker interface
	pricing     map[string]config.PricingInfo // Use config.PricingInfo directly
}

// NewLLMCategorizer creates a new categorizer using an OpenAI-compatible client.
// Accepts optional costStore and pricing for cost tracking.
func NewLLMCategorizer(client interface { // Update signature
	CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}, model, prompt string, costTracker costtracker.CostTracker, pricing map[string]config.PricingInfo) *LLMCategorizer {
	return &LLMCategorizer{
		client: client,
		model:          model,
		promptTemplate: prompt, // Corrected field name
		costTracker:    costTracker, // Corrected variable name
		pricing:        pricing,
	}
}

func (c *LLMCategorizer) Categorize(ctx context.Context, req CategorizationRequest) (CategorizationResult, error) {
	// --- Basic Prompt Construction (Example - replace with proper templating) ---
	prompt := c.promptTemplate
	prompt = strings.ReplaceAll(prompt, "{{TITLE}}", req.Title)
	prompt = strings.ReplaceAll(prompt, "{{BODY}}", req.Body)
	prompt = strings.ReplaceAll(prompt, "{{EXISTING_TAGS}}", strings.Join(req.ExistingTags, ", "))

	if c.client == nil {
		return CategorizationResult{}, fmt.Errorf("LLM categorizer is not initialized with an OpenAI client")
	}

	resp, err := c.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: c.model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return CategorizationResult{}, fmt.Errorf("openai chat completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return CategorizationResult{}, fmt.Errorf("no choices returned from OpenAI")
	}

	content := resp.Choices[0].Message.Content
	content = strings.TrimSpace(content)

	var parsed struct {
		Tags       []string `json:"tags"`
		Category   string   `json:"category"`
		Confidence float64  `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return CategorizationResult{}, fmt.Errorf("failed to parse LLM response as JSON: %w\nResponse content: %s", err, content)
	}

	// Defensive defaults
	if parsed.Confidence == 0 {
		parsed.Confidence = 1.0
	}

	// --- Cost Tracking Instrumentation ---
	if c.costTracker != nil && resp.Usage.TotalTokens > 0 {
		priceInfo, ok := c.pricing[c.model]
		if !ok {
			log.Warnf("Pricing info not found for model '%s'. Cannot record cost for categorization.", c.model)
		} else {
			cost := float64(resp.Usage.PromptTokens)*priceInfo.InputPerToken +
				float64(resp.Usage.CompletionTokens)*priceInfo.OutputPerToken

			costEvent := costtracker.CostEvent{
				Operation:    "categorization",
				AmountUSD:    cost,
				Details: map[string]interface{}{
					"provider_name": "openai", // Assuming OpenAI for now
					"model_name":    c.model,
					"input_tokens":  resp.Usage.PromptTokens,
					"output_tokens": resp.Usage.CompletionTokens,
					"timestamp": time.Now().UTC().Format(time.RFC3339),
					"title": req.Title, // Add some context
					// Add related IDs here if available from the context or request
					// "related_content_id": req.ContentID, // Example if ContentID was part of CategorizationRequest
				},
			}
			if err := c.costTracker.RecordCost(ctx, costEvent); err != nil {
				log.Errorf("Failed to record AI usage log for categorization: %v", err)
			} else {
				log.Debugf("Recorded AI usage: Provider=%s, Service=%s, Model=%s, InputTokens=%d, OutputTokens=%d, Cost=%.8f",
					costEvent.Details["provider_name"],
					costEvent.Operation,
					costEvent.Details["model_name"],
					costEvent.Details["input_tokens"],
					costEvent.Details["output_tokens"],
					costEvent.AmountUSD)
			}

		}
	}
	// --- End Cost Tracking ---

	return CategorizationResult{
		SuggestedTags:     parsed.Tags,
		SuggestedCategory: parsed.Category,
		Confidence:        parsed.Confidence,
	}, nil
}

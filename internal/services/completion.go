package services

import (
	"context"

	"mimir/internal/store" // For ProviderStatus
)

// ChatMessageRole defines the role of the message sender (system, user, assistant).
type ChatMessageRole string

const (
	ChatMessageRoleSystem    ChatMessageRole = "system"
	ChatMessageRoleUser      ChatMessageRole = "user"
	ChatMessageRoleAssistant ChatMessageRole = "assistant" // Or "model" for Gemini
)

// ChatMessage represents a single message in a chat conversation.
type ChatMessage struct {
	Role    ChatMessageRole
	Content string
}

// CompletionService defines the interface for generating text completions or chat responses.
type CompletionService interface {
	GenerateChatCompletion(ctx context.Context, messages []ChatMessage) (string, error)
	Status() store.ProviderStatus // Include status check similar to EmbeddingService
	Name() string                 // Provider name (e.g., "openai", "gemini")
	ModelName() string            // Specific model used
}


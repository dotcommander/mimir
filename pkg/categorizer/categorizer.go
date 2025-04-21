package categorizer

import "context"

// CategorizationRequest holds text + optional context
type CategorizationRequest struct {
	Title        string
	Body         string
	ExistingTags []string
}

// CategorizationResult holds suggested categories
type CategorizationResult struct {
	SuggestedTags     []string
	SuggestedCategory string
	Confidence        float64
}

// ContentCategorizer categorizes content
type ContentCategorizer interface {
	Categorize(ctx context.Context, req CategorizationRequest) (CategorizationResult, error)
}

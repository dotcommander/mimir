package costtracker

import (
	"context"
)

// CostEvent represents a single AI usage event and its cost.
type CostEvent struct {
	Operation string // e.g., "embedding", "summarization", "batch"
	AmountUSD float64
	Details   map[string]interface{}
}

// CostTracker provides methods to record and report costs.
type CostTracker interface {
	RecordCost(ctx context.Context, event CostEvent) error
	TotalCost(ctx context.Context) (float64, error)
	// TODO: Add more reporting methods as needed.
}

// New returns a dummy implementation for now.
func New() CostTracker {
	return &noopCostTracker{}
}

type noopCostTracker struct{}

func (n *noopCostTracker) RecordCost(ctx context.Context, event CostEvent) error { return nil }
func (n *noopCostTracker) TotalCost(ctx context.Context) (float64, error)        { return 0, nil }

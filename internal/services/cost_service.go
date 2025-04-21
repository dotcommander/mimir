package services

import (
	"context"
	"fmt"

	"mimir/internal/models"
	"mimir/internal/store"
)

// CostService provides methods for accessing AI usage cost data.
type CostService struct {
	store store.CostTrackingStore
}

// NewCostService creates a new CostService.
func NewCostService(store store.CostTrackingStore) *CostService {
	return &CostService{store: store}
}

// ListUsage retrieves a paginated list of AI usage logs.
func (s *CostService) ListUsage(ctx context.Context, limit, offset int) ([]*models.AIUsageLog, error) {
	logs, err := s.store.ListUsage(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list usage logs from store: %w", err)
	}
	return logs, nil
}

// GetSummary retrieves the total cost and token usage summary.
func (s *CostService) GetSummary(ctx context.Context) (totalCost float64, totalInputTokens, totalOutputTokens int64, err error) {
	totalCost, totalInputTokens, totalOutputTokens, err = s.store.GetUsageSummary(ctx)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get usage summary from store: %w", err)
	}
	return totalCost, totalInputTokens, totalOutputTokens, nil
}

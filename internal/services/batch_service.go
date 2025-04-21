package services

import (
	"context"
	"fmt"

	"mimir/internal/models"
	"mimir/internal/store"
)

// BatchService handles operations related to batch jobs.
type BatchService struct {
	jobStore store.JobStore
}

// NewBatchService creates a new BatchService.
func NewBatchService(js store.JobStore) *BatchService {
	return &BatchService{
		jobStore: js,
	}
}

// ListBatches retrieves a list of background jobs associated with Batch API calls.
func (s *BatchService) ListBatches(ctx context.Context, limit, offset int) ([]*models.BackgroundJob, error) {
	if limit <= 0 {
		limit = 20 // Default limit
	}
	if offset < 0 {
		offset = 0
	}

	jobs, err := s.jobStore.ListBatchJobs(ctx, limit, offset)
	if err != nil {
		// Wrap the error for context
		return nil, fmt.Errorf("failed to list batch jobs from store: %w", err)
	}
	return jobs, nil
}

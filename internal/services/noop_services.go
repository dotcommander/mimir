package services

import (
	"context"
)

type NoopSummaryService struct{}

// Add contentID and jobID parameters to match the interface
func (s *NoopSummaryService) Summarize(ctx context.Context, text string, contentID int64, jobID string) (string, error) {
	return "", nil
}

type NoopTaggingService struct{}

func (s *NoopTaggingService) SuggestTags(ctx context.Context, text string) ([]string, error) {
	return []string{}, nil
}

// Add these constructor functions at the bottom
func NewNoopSummaryService() SummaryService {
	return &NoopSummaryService{}
}

func NewNoopTaggingService() TaggingService {
	return &NoopTaggingService{}
}

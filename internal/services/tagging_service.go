package services

import (
	"context"
	"errors"
)

// TaggingService suggests tags for content.
type TaggingService interface {
	SuggestTags(ctx context.Context, text string) ([]string, error)
}

// NewTaggingService returns a dummy implementation for now.
func NewTaggingService() TaggingService {
	return &noopTaggingService{}
}

type noopTaggingService struct{}

func (n *noopTaggingService) SuggestTags(ctx context.Context, text string) ([]string, error) {
	if len(text) == 0 {
		return nil, errors.New("cannot suggest tags for empty text")
	}
	// TODO: Replace with actual tag suggestion logic.
	return []string{"tag-placeholder"}, nil
}

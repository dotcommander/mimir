package services

import (
	"context"
	"fmt"

	"mimir/internal/models"
	"mimir/internal/store"
)

type TagService struct {
	store store.TagStore
}

func NewTagService(ts store.TagStore) *TagService {
	return &TagService{store: ts}
}

// TagContent associates the given tag names with the specified content.
// It creates any missing tags, then links them to the content.
func (ts *TagService) TagContent(ctx context.Context, contentID int64, tagNames []string) ([]*models.Tag, error) {
	if len(tagNames) == 0 {
		return []*models.Tag{}, nil
	}

	tags, err := ts.store.GetOrCreateTagsByName(ctx, tagNames)
	if err != nil {
		return nil, fmt.Errorf("get or create tags: %w", err)
	}

	tagIDs := make([]int64, len(tags))
	for i, tag := range tags {
		tagIDs[i] = tag.ID
	}

	err = ts.store.AddTagsToContent(ctx, contentID, tagIDs)
	if err != nil {
		return nil, fmt.Errorf("add tags to content: %w", err)
	}

	return tags, nil
}

// GetContentTags retrieves all tags associated with a specific content ID.
func (ts *TagService) GetContentTags(ctx context.Context, contentID int64) ([]*models.Tag, error) {
	tags, err := ts.store.GetContentTags(ctx, contentID)
	if err != nil {
		// Wrap the error for context
		return nil, fmt.Errorf("failed to get tags for content ID %d from store: %w", contentID, err)
	}
	// Return empty slice, not nil, if no tags found (consistent with store layer)
	if tags == nil {
		return []*models.Tag{}, nil
	}
	return tags, nil
}

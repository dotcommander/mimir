package services

import (
	"context"
	"fmt"
	"log"

	"mimir/internal/models" // Add models import
	"mimir/internal/store"
	categorizer "mimir/pkg/categorizer"
)

type ContentWithCategories struct {
	Tags       []string
	Category   string
	Confidence float64
}

type CategorizationService struct {
	Categorizer       categorizer.ContentCategorizer
	TagService        *TagService
	CollectionService *CollectionService
	contentStore      store.ContentStore
}

func NewCategorizationService(cat categorizer.ContentCategorizer, ts *TagService, cs *CollectionService, contentStore store.ContentStore) *CategorizationService {
	return &CategorizationService{
		Categorizer:       cat,
		TagService:        ts,
		CollectionService: cs,
		contentStore:      contentStore,
	}
}

func (s *CategorizationService) CategorizeContent(ctx context.Context, title, body string, existingTags []string) (*ContentWithCategories, error) {
	res, err := s.Categorizer.Categorize(ctx, categorizer.CategorizationRequest{
		Title:        title,
		Body:         body,
		ExistingTags: existingTags,
	})
	if err != nil {
		return nil, err
	}
	return &ContentWithCategories{
		Tags:       res.SuggestedTags,
		Category:   res.SuggestedCategory,
		Confidence: res.Confidence,
	}, nil
}

func (s *CategorizationService) BatchCategorize(ctx context.Context, contentIDs []int64) (map[int64]*ContentWithCategories, error) {
	results := make(map[int64]*ContentWithCategories, len(contentIDs))
	if len(contentIDs) == 0 {
		return results, nil
	}

	// Fetch all content objects in one go
	contents, err := s.contentStore.GetContentsByIDs(ctx, contentIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch content for batch categorization: %w", err)
	}

	// Fetch all existing tags for these contents in one go
	existingTagsMap, err := s.TagService.store.GetTagsForContents(ctx, contentIDs)
	if err != nil {
		// Log the error but proceed, categorization might still work without existing tags
		log.Printf("WARN: Failed to get existing tags for batch categorize: %v", err)
		existingTagsMap = make(map[int64][]*models.Tag) // Initialize empty map to avoid nil checks later
	}

	// Process each content item
	for _, content := range contents {
		if content == nil {
			log.Printf("WARN: BatchCategorize skipped nil content returned by GetContentsByIDs")
			continue
		}

		// Get existing tags for this specific content ID from the map
		existingModelTags := existingTagsMap[content.ID] // Will be nil or empty slice if not found or no tags
		existingTagNames := make([]string, len(existingModelTags))
		for i, tag := range existingModelTags {
			existingTagNames[i] = tag.Name
		}

		// Perform categorization
		cats, err := s.CategorizeContent(ctx, content.Title, content.Body, existingTagNames)
		if err != nil {
			log.Printf("WARN: Failed to categorize content %d during batch: %v", content.ID, err)
			continue // Skip this content item if categorization fails
		}
		results[content.ID] = cats
	}
	return results, nil
}

// ApplyCategories applies the suggested tags and category (collection) to a content item.
// If autoApply is false, it currently does nothing (future: persist suggestion).

func (s *CategorizationService) ApplyCategories(ctx context.Context, contentID int64, cats *ContentWithCategories, autoApply bool) error {
	if cats == nil {
		return fmt.Errorf("no categories to apply")
	}
	if !autoApply {
		// In future, persist suggestion somewhere
		return nil // Currently does nothing if autoApply is false
	}

	log.Printf("Applying categories for content %d: Tags=%v, Category=%s", contentID, cats.Tags, cats.Category)

	// Apply Tags
	if len(cats.Tags) > 0 {
		if s.TagService == nil {
			log.Printf("WARN: Cannot apply tags for content %d: TagService is nil", contentID)
		} else {
			// TagContent handles getting or creating tags and associating them
			_, err := s.TagService.TagContent(ctx, contentID, cats.Tags)
			if err != nil {
				// Log error but potentially continue to apply collection
				log.Printf("ERROR: Failed to apply tags %v to content %d: %v", cats.Tags, contentID, err)
				// Optionally return the error here if applying tags is critical:
				// return fmt.Errorf("failed to apply tags: %w", err)
			} else {
				log.Printf("Successfully applied tags %v to content %d", cats.Tags, contentID)
			}
		}
	} else {
		log.Printf("No tags to apply for content %d", contentID)
	}

	// Apply Category (as Collection)
	if cats.Category != "" {
		if s.CollectionService == nil {
			log.Printf("WARN: Cannot apply category '%s' for content %d: CollectionService is nil", cats.Category, contentID)
		} else {
			// Get or create the collection corresponding to the category name
			// Use default description (nil) and pinned status (false) for now
			collection, err := s.CollectionService.GetOrCreateCollection(ctx, cats.Category, nil, false)
			if err != nil {
				log.Printf("ERROR: Failed to get or create collection '%s' for content %d: %v", cats.Category, contentID, err)
				// Optionally return the error here if applying collection is critical:
				// return fmt.Errorf("failed to get/create collection: %w", err)
			} else if collection != nil {
				// Add the content to the collection
				err = s.CollectionService.AddContent(ctx, contentID, collection.ID)
				if err != nil {
					// Log error (might fail if already added, which is okay)
					// Check for specific duplicate errors if the store layer returns them
					log.Printf("ERROR: Failed to add content %d to collection '%s' (ID: %d): %v", contentID, collection.Name, collection.ID, err)
					// Optionally return the error:
					// return fmt.Errorf("failed to add content to collection: %w", err)
				} else {
					log.Printf("Successfully added content %d to collection '%s' (ID: %d)", contentID, collection.Name, collection.ID)
				}
			} else {
				// This case should ideally not happen if GetOrCreateCollection works correctly
				log.Printf("ERROR: GetOrCreateCollection returned nil for category '%s', cannot apply to content %d", cats.Category, contentID)
			}
		}
	} else {
		log.Printf("No category/collection to apply for content %d", contentID)
	}

	return nil // Return nil even if some non-critical errors occurred (logged above)
}

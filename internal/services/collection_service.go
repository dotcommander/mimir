package services

import (
	"context"
	"errors"
	"fmt"
	"log"

	"mimir/internal/models"
	"mimir/internal/store"
)

type CollectionService struct {
	collections store.CollectionStore
	contents    store.ContentStore
	tags        store.TagStore
}

func NewCollectionService(collections store.CollectionStore, contents store.ContentStore, tags store.TagStore) *CollectionService {
	return &CollectionService{
		collections: collections,
		contents:    contents,
		tags:        tags,
	}
}

func (cs *CollectionService) CreateCollection(ctx context.Context, name string, description *string, pinned bool) (*models.Collection, error) {
	if name == "" {
		return nil, fmt.Errorf("collection name cannot be empty")
	}
	if description != nil && *description == "" {
		description = nil
	}

	c := &models.Collection{
		Name:        name,
		Description: description,
		IsPinned:    pinned,
	}
	if err := cs.collections.CreateCollection(ctx, c); err != nil {
		return nil, fmt.Errorf("could not create collection: %w", err)
	}
	return c, nil
}

// GetOrCreateCollection finds a collection by name or creates it if it doesn't exist.
func (cs *CollectionService) GetOrCreateCollection(ctx context.Context, name string, description *string, pinned bool) (*models.Collection, error) {
	if name == "" {
		return nil, fmt.Errorf("collection name cannot be empty")
	}

	existingColl, err := cs.collections.GetCollectionByName(ctx, name)
	if err == nil {
		// Collection found, return it
		return existingColl, nil
	}

	// If not found, create it
	if errors.Is(err, store.ErrNotFound) {
		if description != nil && *description == "" {
			description = nil
		}
		return cs.CreateCollection(ctx, name, description, pinned)
	}

	// Any other error during lookup
	return nil, fmt.Errorf("failed to get collection '%s': %w", name, err)
}

func (cs *CollectionService) ListCollections(ctx context.Context) ([]*models.Collection, error) {
	return cs.collections.ListCollections(ctx, 100, 0, nil)
}

func (cs *CollectionService) AddContent(ctx context.Context, contentID, collectionID int64) error {
	return cs.collections.AddContentToCollection(ctx, collectionID, contentID)
}

func (cs *CollectionService) RemoveContent(ctx context.Context, contentID, collectionID int64) error {
	return cs.collections.RemoveContentFromCollection(ctx, collectionID, contentID)
}

func (cs *CollectionService) ListContent(ctx context.Context, collectionID int64, limit, offset int, sortBy, sortOrder string) ([]ContentResultItem, error) {
	contents, err := cs.collections.ListContentByCollection(ctx, collectionID, limit, offset, sortBy, sortOrder)
	if err != nil {
		return nil, err
	}

	result := make([]ContentResultItem, len(contents))
	for i, c := range contents {
		tags, err := cs.tags.GetContentTags(ctx, c.ID)
		if err != nil {
			log.Printf("WARN: Failed to get tags for content %d in collection %d: %v", c.ID, collectionID, err)
		}
		result[i] = ContentResultItem{Content: *c, Tags: tags}
	}
	return result, nil
}

// GetCollection retrieves a single collection by its ID.
// --- REMOVING DUPLICATE METHOD DEFINITIONS BELOW ---
/*
func (cs *CollectionService) GetCollection(ctx context.Context, id int64) (*models.Collection, error) {
	coll, err := cs.collections.GetCollection(ctx, id)
	if err != nil {
		// Wrap error, potentially check for store.ErrNotFound
		return nil, fmt.Errorf("failed to get collection %d: %w", id, err)
	}
	return coll, nil
}

// ListCollections retrieves a list of collections, optionally filtering by pinned status.
func (cs *CollectionService) ListCollections(ctx context.Context, limit, offset int, pinned *bool) ([]*models.Collection, error) {
	if limit <= 0 {
		limit = 20 // Default limit
	}
	if offset < 0 {
		offset = 0
	}
	collections, err := cs.collections.ListCollections(ctx, limit, offset, pinned)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}
	return collections, nil
}

// UpdateCollection updates an existing collection's details.
// Pass nil for fields that should not be updated.
func (cs *CollectionService) UpdateCollection(ctx context.Context, id int64, name *string, description *string, isPinned *bool) (*models.Collection, error) {
	// Fetch the existing collection first to ensure it exists
	existingColl, err := cs.collections.GetCollection(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection %d for update: %w", id, err)
	}

	// Apply updates if provided
	updated := false
	if name != nil && *name != "" && *name != existingColl.Name {
		existingColl.Name = *name
		updated = true
	}
	// Handle nil description carefully
	// If new description is provided and different from existing (or existing was nil)
	if description != nil && (existingColl.Description == nil || *description != *existingColl.Description) {
		existingColl.Description = description
		updated = true
	}
	// If new description is explicitly nil and existing was not nil
	if description == nil && existingColl.Description != nil {
		existingColl.Description = nil
		updated = true
	}

	if isPinned != nil && *isPinned != existingColl.IsPinned {
		existingColl.IsPinned = *isPinned
		updated = true
	}

	// Only call update if something actually changed
	if updated {
		err = cs.collections.UpdateCollection(ctx, existingColl)
		if err != nil {
			return nil, fmt.Errorf("failed to update collection %d: %w", id, err)
		}
	}

	return existingColl, nil
}

// DeleteCollection removes a collection.
func (cs *CollectionService) DeleteCollection(ctx context.Context, id int64) error {
	err := cs.collections.DeleteCollection(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete collection %d: %w", id, err)
	}
	return nil
}
*/
// --- END REMOVED DUPLICATE METHODS ---

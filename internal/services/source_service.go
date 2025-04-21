package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"

	"mimir/internal/models"
	"mimir/internal/store"
)

type GetOrCreateSourceParams struct {
	Name string
	Type string
	Desc *string
	URL  *string
}

// GetOrCreateDirectorySource creates or retrieves a source for a directory.
func (s *SourceService) GetOrCreateDirectorySource(ctx context.Context, dirPath string) (*models.Source, error) {
	baseName := filepath.Base(dirPath)
	desc := fmt.Sprintf("Auto-created for directory: %s", dirPath)
	url := fmt.Sprintf("file://%s", dirPath)
	return s.GetOrCreateSource(ctx, GetOrCreateSourceParams{
		Name: baseName,
		Type: "directory",
		Desc: &desc,
		URL:  &url,
	})
}

type CreateSourceParams struct {
	Name string
	Type string
	Desc *string
	URL  *string
}

// SourceService encapsulates source CRUD logic
type SourceService struct {
	primaryStore store.SourceStore
}

func NewSourceService(ps store.SourceStore) *SourceService {
	return &SourceService{primaryStore: ps}
}

func (s *SourceService) GetOrCreateSource(ctx context.Context, params GetOrCreateSourceParams) (*models.Source, error) {
	if params.Name == "" {
		return nil, fmt.Errorf("source name cannot be empty")
	}

	src, err := s.primaryStore.GetSourceByName(ctx, params.Name)
	if err == nil {
		// Source found, return it
		return src, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		// An unexpected error occurred while trying to get the source
		return nil, fmt.Errorf("failed to get source '%s': %w", params.Name, err)
	}

	// Source not found, proceed to create it
	stype := params.Type
	if stype == "" {
		log.Printf("WARN: Creating source '%s' with default type 'unknown'", params.Name)
		stype = "unknown"
	}
	descPtr := params.Desc
	if descPtr != nil && *descPtr == "" {
		descPtr = nil
	}
	urlPtr := params.URL
	if urlPtr != nil && *urlPtr == "" {
		urlPtr = nil
	}
	source := &models.Source{
		Name:        params.Name,
		SourceType:  stype,
		Description: descPtr,
		URL:         urlPtr,
	}
	err = s.primaryStore.CreateSource(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("failed to create source '%s': %w", params.Name, err)
	}
	return source, nil
}

func (s *SourceService) CreateSource(ctx context.Context, name, sourceType string, description, url *string) (*models.Source, error) {
	if name == "" {
		return nil, fmt.Errorf("source name cannot be empty")
	}
	if sourceType == "" {
		return nil, fmt.Errorf("source type cannot be empty")
	}
	if description != nil && *description == "" {
		description = nil
	}
	if url != nil && *url == "" {
		url = nil
	}

	if name == "" {
		return nil, fmt.Errorf("source name cannot be empty")
	}
	if sourceType == "" {
		return nil, fmt.Errorf("source type cannot be empty")
	}
	source := &models.Source{
		Name:        name,
		SourceType:  sourceType,
		Description: description,
		URL:         url,
	}
	err := s.primaryStore.CreateSource(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("failed to create source: %w", err)
	}
	return source, nil
}

func (s *SourceService) ListSources(ctx context.Context) ([]*models.Source, error) {
	sources, err := s.primaryStore.ListSources(ctx, 100, 0)
	if err != nil {
		return nil, fmt.Errorf("could not list sources: %w", err)
	}
	return sources, nil
}

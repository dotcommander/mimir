package apihandlers

import (
	"mimir/internal/app"
	"mimir/internal/models"
)

// SearchResultItem represents a search result with content, score, and tags.
type SearchResultItem struct {
	Content *models.Content
	Score   float64
	Tags    []*models.Tag
}

// SearchContent is a stub implementation to satisfy tests.
// Replace with real implementation later.
func SearchContent(app *app.App, query string, limit int, filterTags []string) ([]SearchResultItem, error) {
	// Return empty slice and nil error for now
	return []SearchResultItem{}, nil
}

// KeywordSearchContent is a stub implementation to satisfy tests.
// Replace with real implementation later.
func KeywordSearchContent(app *app.App, query string, filterTags []string) ([]*models.Content, error) {
	// Return empty slice and nil error for now
	return []*models.Content{}, nil
}

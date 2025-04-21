package primary

import (
	"context"
	"errors"
	"fmt"
	"time"

	"mimir/internal/models"
	"mimir/internal/store"

	"github.com/jackc/pgx/v5"
)

// --- Search History Store Implementation ---

func (s *StoreImpl) RecordSearchQuery(ctx context.Context, query string, resultsCount int) (*models.SearchQuery, error) {
	sql := `
		INSERT INTO search_queries (query, results_count, executed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $3, $3)
		RETURNING id, executed_at, created_at, updated_at`

	now := time.Now()
	searchQuery := &models.SearchQuery{
		Query:        query,
		ResultsCount: resultsCount,
	}

	err := s.db.QueryRow(ctx, sql, query, resultsCount, now).Scan(
		&searchQuery.ID, &searchQuery.ExecutedAt, &searchQuery.CreatedAt, &searchQuery.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to record search query: %w", err)
	}
	return searchQuery, nil
}

func (s *StoreImpl) ListSearchQueries(ctx context.Context, limit int) ([]*models.SearchQuery, error) {
	if limit <= 0 {
		limit = 20 // Default limit
	}
	sql := `
		SELECT id, query, results_count, executed_at, created_at, updated_at
		FROM search_queries
		ORDER BY executed_at DESC
		LIMIT $1`

	rows, err := s.db.Query(ctx, sql, limit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []*models.SearchQuery{}, nil // Return empty slice if no history
		}
		return nil, fmt.Errorf("failed to list search queries: %w", err)
	}
	defer rows.Close()

	var queries []*models.SearchQuery
	for rows.Next() {
		q := &models.SearchQuery{}
		err := rows.Scan(
			&q.ID, &q.Query, &q.ResultsCount, &q.ExecutedAt, &q.CreatedAt, &q.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search query row: %w", err)
		}
		queries = append(queries, q)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search query rows: %w", err)
	}

	return queries, nil
}

func (s *StoreImpl) RecordSearchResults(ctx context.Context, queryID int64, results []models.SearchResult) error {
	// Use a transaction for batch insert
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction for recording search results: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback if commit fails

	sql := `
		INSERT INTO search_results (search_query_id, content_id, relevance_score, rank, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)`
	now := time.Now()

	for _, res := range results {
		_, err := tx.Exec(ctx, sql,
			queryID, res.ContentID, res.RelevanceScore, res.Rank, now,
		)
		if err != nil {
			// Consider logging the specific result that failed
			return fmt.Errorf("failed to insert search result for query %d, content %d: %w", queryID, res.ContentID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction for recording search results: %w", err)
	}

	return nil
}

// Ensure StoreImpl satisfies the SearchHistoryStore interface
var _ store.SearchHistoryStore = (*StoreImpl)(nil)

package primary

import (
	"context"
	"errors"
	"fmt"
	"time"

	"mimir/internal/models"
	"mimir/internal/store"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// --- Source Management ---

func (s *StoreImpl) CreateSource(ctx context.Context, source *models.Source) error {
	query := `
		INSERT INTO sources (name, description, url, source_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`

	now := time.Now()
	err := s.db.QueryRow(ctx, query,
		source.Name, source.Description, source.URL, source.SourceType, now, now,
	).Scan(&source.ID, &source.CreatedAt, &source.UpdatedAt)

	if err != nil {
		// Handle potential unique constraint violation on 'name' if needed
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			// Consider returning a specific error type for duplicate names
			return fmt.Errorf("source with name '%s' already exists: %w", source.Name, store.ErrDuplicate)
		}
		return fmt.Errorf("failed to insert source: %w", err)
	}
	return nil
}

func (s *StoreImpl) GetSource(ctx context.Context, id int64) (*models.Source, error) {
	query := `SELECT id, name, description, url, source_type, created_at, updated_at FROM sources WHERE id = $1`
	source := &models.Source{}
	err := s.db.QueryRow(ctx, query, id).Scan(
		&source.ID, &source.Name, &source.Description, &source.URL, &source.SourceType, &source.CreatedAt, &source.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get source by id %d: %w", id, err)
	}
	return source, nil
}

func (s *StoreImpl) GetSourceByName(ctx context.Context, name string) (*models.Source, error) {
	query := `SELECT id, name, description, url, source_type, created_at, updated_at FROM sources WHERE name = $1`
	source := &models.Source{}
	err := s.db.QueryRow(ctx, query, name).Scan(
		&source.ID, &source.Name, &source.Description, &source.URL, &source.SourceType, &source.CreatedAt, &source.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get source by name '%s': %w", name, err)
	}
	return source, nil
}

func (s *StoreImpl) ListSources(ctx context.Context, limit int, offset int) ([]*models.Source, error) {
	query := `SELECT id, name, description, url, source_type, created_at, updated_at FROM sources ORDER BY name ASC LIMIT $1 OFFSET $2`
	if limit <= 0 {
		limit = 20 // Default limit
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}
	defer rows.Close()

	var sources []*models.Source
	for rows.Next() {
		source := &models.Source{}
		err := rows.Scan(
			&source.ID, &source.Name, &source.Description, &source.URL, &source.SourceType, &source.CreatedAt, &source.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan source row: %w", err)
		}
		sources = append(sources, source)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating source rows: %w", err)
	}

	return sources, nil
}

// Ensure StoreImpl satisfies the SourceStore interface
var _ store.SourceStore = (*StoreImpl)(nil)

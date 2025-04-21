package primary

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mimir/internal/models"
)

// StoreImpl implements the store.PrimaryStore interface using PostgreSQL.
type StoreImpl struct {
	db *pgxpool.Pool
}

// NewPrimaryStore creates a new PostgreSQL primary store implementation.
func NewPrimaryStore(ctx context.Context, dsn string) (*StoreImpl, error) {
	if dsn == "" {
		return nil, errors.New("database DSN cannot be empty")
	}
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database DSN: %w", err)
	}

	dbpool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := dbpool.Ping(ctx); err != nil {
		dbpool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return &StoreImpl{db: dbpool}, nil
}

// Ping checks the database connection.
func (s *StoreImpl) Ping(ctx context.Context) error {
	return s.db.Ping(ctx)
}

// Close closes the database connection pool.
func (s *StoreImpl) Close() {
	s.db.Close()
}

// --- Helper Functions ---

// scanContent scans a single row from pgx.Rows into a models.Content struct.
// It expects the columns in the order defined by queries selecting content fields.
func scanContent(rows pgx.Rows, dest *models.Content) error {
	// Ensure the order matches the SELECT statement in functions like KeywordSearchContent
	return rows.Scan(
		&dest.ID,
		&dest.SourceID,
		&dest.Title,
		&dest.Body,
		&dest.ContentHash,
		&dest.FilePath,
		&dest.FileSize,
		&dest.ContentType,
		&dest.Metadata,
		&dest.EmbeddingID,
		&dest.IsEmbedded,
		&dest.LastAccessedAt,
		&dest.ModifiedAt,
		&dest.Summary,
		&dest.CreatedAt,
		&dest.UpdatedAt,
	)
}

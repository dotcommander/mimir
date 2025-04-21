package vector

import (
	"encoding/json" // Add json import
	"context"
	"errors" // Add missing import
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mimir/internal/models"
	"mimir/internal/store"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

// VectorSearchResultItem holds detailed results from a vector search, including chunk info.
// Note: This is defined locally as it's specific to the vector store's output format.
type VectorSearchResultItem struct {
	ContentID      int64
	RelevanceScore float64
	ChunkText      string          // The text of the matched chunk
	Metadata       json.RawMessage // Raw JSON metadata of the matched chunk
}

type StoreImpl struct {
	db *pgxpool.Pool
}

func NewStore(ctx context.Context, dsn string) (store.VectorStore, error) {
	if dsn == "" {
		return nil, fmt.Errorf("vector store DSN cannot be empty")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vector store DSN: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgx pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping vector store: %w", err)
	}
	log.Printf("Successfully connected to PostgreSQL vector store.")
	return &StoreImpl{db: pool}, nil
}

func (vs *StoreImpl) Close() error {
	if vs.db != nil {
		log.Println("Closing PostgreSQL vector store connection...")
		vs.db.Close()
	}
	return nil
}

func (vs *StoreImpl) Ping(ctx context.Context) error {
	if vs.db == nil {
		return fmt.Errorf("vector store connection is not initialized")
	}
	return vs.db.Ping(ctx)
}

func (vs *StoreImpl) AddEmbedding(ctx context.Context, entry *models.EmbeddingEntry) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	// Add chunk_text to INSERT query
	query := `INSERT INTO embeddings (id, content_id, chunk_text, vector, metadata) VALUES ($1, $2, $3, $4, $5) RETURNING created_at`
	err := vs.db.QueryRow(ctx, query, entry.ID, entry.ContentID, entry.ChunkText, pgvector.NewVector(entry.Vector.Slice()), entry.Metadata).Scan(&entry.CreatedAt)
	if err != nil {
		return fmt.Errorf("add embedding: %w", err)
	}
	return nil
}

func (vs *StoreImpl) GetEmbedding(ctx context.Context, id uuid.UUID) (*models.EmbeddingEntry, error) {
	// Add chunk_text to SELECT query
	query := `SELECT id, content_id, chunk_text, vector, metadata, created_at FROM embeddings WHERE id = $1`
	entry := &models.EmbeddingEntry{}
	var vector pgvector.Vector // Renamed variable
	err := vs.db.QueryRow(ctx, query, id).Scan(&entry.ID, &entry.ContentID, &entry.ChunkText, &vector, &entry.Metadata, &entry.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { // Use errors.Is for pgx v5+
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("get embedding: %w", err)
	}
	entry.Vector = vector // Assign to the correct field
	return entry, nil
}

func (vs *StoreImpl) DeleteEmbeddingsByContentID(ctx context.Context, contentID int64) error {
	query := `DELETE FROM embeddings WHERE content_id = $1`
	_, err := vs.db.Exec(ctx, query, contentID)
	if err != nil {
		return fmt.Errorf("delete embeddings: %w", err)
	}
	return nil
}

func (vs *StoreImpl) SimilaritySearch(ctx context.Context, queryVector pgvector.Vector, k int, filterMetadata map[string]interface{}) ([]models.SearchResult, error) {
	if len(filterMetadata) > 0 {
		log.Println("WARN: Metadata filtering not yet implemented for pgvector SimilaritySearch")
	}

	// Select chunk_text and metadata explicitly
	query := `SELECT id, content_id, chunk_text, (vector <-> $1) as score, metadata, created_at
             FROM embeddings ORDER BY vector <-> $1 LIMIT $2`

	rows, err := vs.db.Query(ctx, query, queryVector, k)
	if err != nil {
		return nil, fmt.Errorf("similarity search query: %w", err)
	}
	defer rows.Close()

	var results []models.SearchResult
	for rows.Next() {
		var entry models.EmbeddingEntry // Use to scan easily
		var score float64
		// Scan chunk_text and metadata
		if err := rows.Scan(&entry.ID, &entry.ContentID, &entry.ChunkText, &score, &entry.Metadata, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan similarity search row: %w", err)
		}
		results = append(results, models.SearchResult{
			ContentID:      entry.ContentID,
			RelevanceScore: score,
			// ChunkText and Metadata are not part of models.SearchResult
			// Rank needs to be assigned later if needed
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate similarity search rows: %w", err)
	}
	return results, nil
}

package primary

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"mimir/internal/models"
	"mimir/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// --- Content Management ---

// calculateHash generates a SHA256 hash for the content body.
func calculateHash(body string) string {
	hasher := sha256.New()
	hasher.Write([]byte(body))
	return hex.EncodeToString(hasher.Sum(nil))
}

// CreateContent inserts a new content record.
// Note: This basic version doesn't check for duplicates by hash.
// Use CreateContentIfNotExists for that behavior.
func (s *StoreImpl) CreateContent(ctx context.Context, content *models.Content) error {
	query := `
		INSERT INTO content (
			source_id, title, body, content_hash, 
			file_path, file_size, content_type, 
			metadata, summary, created_at, updated_at, modified_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at`

	now := time.Now()
	content.ContentHash = calculateHash(content.Body) // Calculate hash before insert
	if content.Metadata == nil {
		content.Metadata = json.RawMessage("{}") // Default to empty JSON object
	}

	err := s.db.QueryRow(ctx, query,
		content.SourceID, content.Title, content.Body, content.ContentHash,
		content.FilePath, content.FileSize, content.ContentType, content.Metadata,
		content.Summary, now, now, content.ModifiedAt,
	).Scan(&content.ID, &content.CreatedAt, &content.UpdatedAt)

	if err != nil {
		// Handle potential foreign key violation if source_id doesn't exist
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			return fmt.Errorf("source ID %d does not exist: %w", content.SourceID, store.ErrForeignKeyViolation)
		}
		// Handle potential unique constraint violation on content_hash if added
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			// This basic CreateContent shouldn't hit this if hash constraint exists,
			// but handle defensively. Use CreateContentIfNotExists for hash checks.
			return fmt.Errorf("content with hash %s already exists (use CreateContentIfNotExists): %w", content.ContentHash, store.ErrDuplicate)
		}
		return fmt.Errorf("failed to insert content: %w", err)
	}
	return nil
}

// CreateContentIfNotExists checks for existing content by hash before inserting.
// Returns true if content already existed (based on hash), false otherwise.
func (s *StoreImpl) CreateContentIfNotExists(ctx context.Context, content *models.Content) (bool, error) {
	content.ContentHash = calculateHash(content.Body)
	existing, err := s.FindContentByHash(ctx, content.ContentHash)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return false, fmt.Errorf("failed checking for existing content by hash: %w", err)
	}
	if existing != nil {
		// Content with the same hash exists, update the passed content object with existing data
		*content = *existing
		return true, nil // Indicate content existed
	}

	// Content doesn't exist, create it
	err = s.CreateContent(ctx, content)
	if err != nil {
		// Handle potential race condition if another process inserted between check and create
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation on hash
			// Re-fetch the content that was just inserted by the other process
			existing, errFetch := s.FindContentByHash(ctx, content.ContentHash)
			if errFetch != nil {
				return false, fmt.Errorf("failed to fetch concurrently inserted content (hash %s): %w", content.ContentHash, errFetch)
			}
			if existing == nil {
				// This case is highly unlikely but possible
				return false, fmt.Errorf("unique constraint violation for hash %s, but content not found on re-fetch", content.ContentHash)
			}
			*content = *existing
			return true, nil // Indicate content existed (inserted by another process)
		}
		// Other insertion error
		return false, fmt.Errorf("failed to create new content: %w", err)
	}

	return false, nil // Indicate content was newly created
}

func (s *StoreImpl) GetContent(ctx context.Context, id int64) (*models.Content, error) {
	query := `
		SELECT id, source_id, title, body, content_hash, 
			   file_path, file_size, content_type, metadata, 
			   summary, is_embedded, embedding_id, created_at, updated_at, modified_at
		FROM content
		WHERE id = $1`
	content := &models.Content{}
	err := s.db.QueryRow(ctx, query, id).Scan(
		&content.ID, &content.SourceID, &content.Title, &content.Body, &content.ContentHash,
		&content.FilePath, &content.FileSize, &content.ContentType, &content.Metadata,
		&content.Summary, &content.IsEmbedded, &content.EmbeddingID, &content.CreatedAt, &content.UpdatedAt,
		&content.ModifiedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get content by id %d: %w", id, err)
	}
	return content, nil
}

func (s *StoreImpl) GetContentsByIDs(ctx context.Context, ids []int64) ([]*models.Content, error) {
	if len(ids) == 0 {
		return []*models.Content{}, nil
	}

	query := `
		SELECT id, source_id, title, body, content_hash, 
			   file_path, file_size, content_type, metadata, 
			   summary, is_embedded, embedding_id, created_at, updated_at, modified_at
		FROM content
		WHERE id = ANY($1)` // Use ANY for efficient lookup

	rows, err := s.db.Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to query contents by IDs: %w", err)
	}
	defer rows.Close()

	contentsMap := make(map[int64]*models.Content)
	for rows.Next() {
		content := &models.Content{}
		err := rows.Scan(
			&content.ID, &content.SourceID, &content.Title, &content.Body, &content.ContentHash,
			&content.FilePath, &content.FileSize, &content.ContentType, &content.Metadata,
			&content.Summary, &content.IsEmbedded, &content.EmbeddingID, &content.CreatedAt, &content.UpdatedAt,
			&content.ModifiedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed scanning content row: %w", err)
		}
		contentsMap[content.ID] = content
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating content rows: %w", err)
	}

	// Return results in the order of the input IDs, handling missing items
	results := make([]*models.Content, len(ids))
	for i, id := range ids {
		results[i] = contentsMap[id] // Will be nil if not found
	}

	return results, nil
}

func (s *StoreImpl) UpdateContent(ctx context.Context, content *models.Content) error {
	query := `
		UPDATE content SET
			title = $1,
			body = $2,
			content_hash = $3,
			file_path = $4,
			file_size = $5,
			content_type = $6,
			metadata = $7,
			summary = $8,
			updated_at = $9,
			modified_at = $10
		WHERE id = $11
		RETURNING updated_at`

	now := time.Now()
	content.ContentHash = calculateHash(content.Body) // Recalculate hash on update
	if content.Metadata == nil {
		content.Metadata = json.RawMessage("{}")
	}

	err := s.db.QueryRow(ctx, query,
		content.Title, content.Body, content.ContentHash,
		content.FilePath, content.FileSize, content.ContentType,
		content.Metadata, content.Summary, now, content.ModifiedAt, content.ID,
	).Scan(&content.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.ErrNotFound // Content ID didn't exist
		}
		// Handle potential unique constraint violation on content_hash if added
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("content with hash %s already exists: %w", content.ContentHash, store.ErrDuplicate)
		}
		return fmt.Errorf("failed to update content %d: %w", content.ID, err)
	}
	return nil
}

func (s *StoreImpl) DeleteContent(ctx context.Context, id int64) error {
	// We might need to delete associated tags first if foreign keys have ON DELETE RESTRICT
	// Delete associations from content_tags
	tagQuery := `DELETE FROM content_tags WHERE content_id = $1`
	_, err := s.db.Exec(ctx, tagQuery, id)
	if err != nil {
		// Log error but continue to delete content? Or return error?
		// Returning error is safer to ensure data integrity.
		return fmt.Errorf("delete content: failed to delete tag associations for content %d: %w", id, err)
	}

	// Delete associations from collection_content
	collQuery := `DELETE FROM collection_content WHERE content_id = $1`
	_, err = s.db.Exec(ctx, collQuery, id)
	if err != nil {
		return fmt.Errorf("delete content: failed to delete collection associations for content %d: %w", id, err)
	}

	// Now delete the content itself
	query := `DELETE FROM content WHERE id = $1`
	commandTag, err := s.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete content: failed to delete content %d: %w", id, err)
	}
	if commandTag.RowsAffected() == 0 {
		return store.ErrNotFound // Content ID didn't exist
	}
	return nil
}

func (s *StoreImpl) ListContent(ctx context.Context, limit, offset int, sortBy, sortOrder string, filterTags []string) ([]*models.Content, error) {
	baseQuery := `
		SELECT DISTINCT c.id, c.source_id, c.title, c.body, c.content_hash, 
						c.file_path, c.file_size, c.content_type, c.metadata, 
						c.summary, c.is_embedded, c.embedding_id, c.created_at, c.updated_at, c.modified_at
		FROM content c`
	var joinClause string
	var whereClause string
	args := []interface{}{}
	argID := 1

	// Filtering by tags
	if len(filterTags) > 0 {
		joinClause = ` JOIN content_tags ct ON c.id = ct.content_id JOIN tags t ON ct.tag_id = t.id`
		placeholders := make([]string, len(filterTags))
		for i, tag := range filterTags {
			placeholders[i] = fmt.Sprintf("$%d", argID)
			args = append(args, tag) // Use tag name or slug depending on desired filtering
			argID++
		}
		// Assuming filtering by tag name here
		whereClause = fmt.Sprintf(" WHERE t.name IN (%s)", strings.Join(placeholders, ","))
		// If filtering by slug: WHERE t.slug IN (...)
	}

	// Sorting
	validSortColumns := map[string]bool{"c.id": true, "c.title": true, "c.created_at": true, "c.updated_at": true, "c.modified_at": true} // Add modified_at
	if !validSortColumns[sortBy] {
		sortBy = "c.created_at" // Default sort column
	}
	sortOrder = strings.ToUpper(sortOrder)
	if sortOrder != "ASC" && sortOrder != "DESC" {
		sortOrder = "DESC" // Default sort order
	}
	orderByClause := fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	// Pagination
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	limitClause := fmt.Sprintf(" LIMIT $%d OFFSET $%d", argID, argID+1)
	args = append(args, limit, offset)

	// Combine query parts
	fullQuery := baseQuery + joinClause + whereClause + orderByClause + limitClause

	rows, err := s.db.Query(ctx, fullQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list content: %w", err)
	}
	defer rows.Close()

	var contents []*models.Content
	for rows.Next() {
		content := &models.Content{}
		err := rows.Scan(
			&content.ID, &content.SourceID, &content.Title, &content.Body, &content.ContentHash,
			&content.FilePath, &content.FileSize, &content.ContentType, &content.Metadata,
			&content.Summary, &content.IsEmbedded, &content.EmbeddingID, &content.CreatedAt, &content.UpdatedAt,
			&content.ModifiedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan content row: %w", err)
		}
		contents = append(contents, content)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating content rows: %w", err)
	}

	return contents, nil
}

func (s *StoreImpl) FindContentByHash(ctx context.Context, hash string) (*models.Content, error) {
	query := `
		SELECT id, source_id, title, body, content_hash, 
			   file_path, file_size, content_type, metadata, 
			   summary, is_embedded, embedding_id, created_at, updated_at, modified_at
		FROM content
		WHERE content_hash = $1`
	content := &models.Content{}
	err := s.db.QueryRow(ctx, query, hash).Scan(
		&content.ID, &content.SourceID, &content.Title, &content.Body, &content.ContentHash,
		&content.FilePath, &content.FileSize, &content.ContentType, &content.Metadata,
		&content.Summary, &content.IsEmbedded, &content.EmbeddingID, &content.CreatedAt, &content.UpdatedAt,
		&content.ModifiedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("failed to find content by hash %s: %w", hash, err)
	}
	return content, nil
}

func (s *StoreImpl) UpdateContentEmbeddingStatus(ctx context.Context, contentID int64, embeddingID uuid.UUID, isEmbedded bool) error {
	query := `UPDATE content SET is_embedded = $1, embedding_id = $2, updated_at = $3 WHERE id = $4`
	now := time.Now()
	commandTag, err := s.db.Exec(ctx, query, isEmbedded, embeddingID, now, contentID)
	if err != nil {
		return fmt.Errorf("failed to update embedding status for content %d: %w", contentID, err)
	}
	if commandTag.RowsAffected() == 0 {
		return store.ErrNotFound // Content ID didn't exist
	}
	return nil
}

// Ensure StoreImpl satisfies the ContentStore interface
var _ store.ContentStore = (*StoreImpl)(nil)

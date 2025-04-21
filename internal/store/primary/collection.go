package primary

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"mimir/internal/models"
	"mimir/internal/store"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// --- Collection Management ---

func (s *StoreImpl) CreateCollection(ctx context.Context, collection *models.Collection) error {
	query := `
		INSERT INTO collections (name, description, is_pinned, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`

	now := time.Now()
	err := s.db.QueryRow(ctx, query,
		collection.Name, collection.Description, collection.IsPinned, now, now,
	).Scan(&collection.ID, &collection.CreatedAt, &collection.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return fmt.Errorf("collection with name '%s' already exists: %w", collection.Name, store.ErrDuplicate)
		}
		return fmt.Errorf("failed to insert collection: %w", err)
	}
	return nil
}

func (s *StoreImpl) GetCollection(ctx context.Context, id int64) (*models.Collection, error) {
	query := `SELECT id, name, description, is_pinned, created_at, updated_at FROM collections WHERE id = $1`
	c := &models.Collection{}
	err := s.db.QueryRow(ctx, query, id).Scan(
		&c.ID, &c.Name, &c.Description, &c.IsPinned, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get collection by id %d: %w", id, err)
	}
	return c, nil
}

func (s *StoreImpl) GetCollectionByName(ctx context.Context, name string) (*models.Collection, error) {
	query := `SELECT id, name, description, is_pinned, created_at, updated_at FROM collections WHERE name = $1`
	c := &models.Collection{}
	err := s.db.QueryRow(ctx, query, name).Scan(
		&c.ID, &c.Name, &c.Description, &c.IsPinned, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get collection by name '%s': %w", name, err)
	}
	return c, nil
}

func (s *StoreImpl) ListCollections(ctx context.Context, limit, offset int, pinned *bool) ([]*models.Collection, error) {
	query := `SELECT id, name, description, is_pinned, created_at, updated_at FROM collections`
	args := []interface{}{}
	whereClause := ""
	if pinned != nil {
		whereClause = " WHERE is_pinned = $1"
		args = append(args, *pinned)
	}
	query += whereClause + " ORDER BY name ASC LIMIT $2 OFFSET $3"
	args = append(args, limit, offset)

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}
	defer rows.Close()

	var collections []*models.Collection
	for rows.Next() {
		c := &models.Collection{}
		err := rows.Scan(
			&c.ID, &c.Name, &c.Description, &c.IsPinned, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan collection row: %w", err)
		}
		collections = append(collections, c)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating collection rows: %w", err)
	}
	return collections, nil
}

func (s *StoreImpl) UpdateCollection(ctx context.Context, collection *models.Collection) error {
	query := `
		UPDATE collections
		SET name = $1, description = $2, is_pinned = $3, updated_at = $4
		WHERE id = $5
		RETURNING updated_at`

	now := time.Now()
	err := s.db.QueryRow(ctx, query,
		collection.Name, collection.Description, collection.IsPinned, now, collection.ID,
	).Scan(&collection.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("collection with name '%s' already exists: %w", collection.Name, store.ErrDuplicate)
		}
		return fmt.Errorf("failed to update collection %d: %w", collection.ID, err)
	}
	return nil
}

func (s *StoreImpl) DeleteCollection(ctx context.Context, id int64) error {
	// First delete collection_content associations
	queryAssoc := `DELETE FROM collection_content WHERE collection_id = $1`
	_, err := s.db.Exec(ctx, queryAssoc, id)
	if err != nil {
		return fmt.Errorf("failed to delete collection-content associations for collection %d: %w", id, err)
	}

	// Then delete the collection itself
	query := `DELETE FROM collections WHERE id = $1`
	cmdTag, err := s.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete collection %d: %w", id, err)
	}
	if cmdTag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (s *StoreImpl) AddContentToCollection(ctx context.Context, collectionID, contentID int64) error {
	query := `
		INSERT INTO collection_content (collection_id, content_id, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`
	_, err := s.db.Exec(ctx, query, collectionID, contentID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to add content %d to collection %d: %w", contentID, collectionID, err)
	}
	return nil
}

func (s *StoreImpl) RemoveContentFromCollection(ctx context.Context, collectionID, contentID int64) error {
	query := `DELETE FROM collection_content WHERE collection_id = $1 AND content_id = $2`
	_, err := s.db.Exec(ctx, query, collectionID, contentID)
	if err != nil {
		return fmt.Errorf("failed to remove content %d from collection %d: %w", contentID, collectionID, err)
	}
	return nil
}

func (s *StoreImpl) GetCollectionContent(ctx context.Context, collectionID int64, limit, offset int) ([]*models.Content, error) {
	query := `
		SELECT c.id, c.source_id, c.title, c.body, c.content_hash, c.file_path, c.file_size, c.content_type, c.metadata, c.is_embedded, c.embedding_id, c.created_at, c.updated_at
		FROM contents c
		JOIN collection_content cc ON c.id = cc.content_id
		WHERE cc.collection_id = $1
		ORDER BY c.created_at DESC
		LIMIT $2 OFFSET $3`

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.Query(ctx, query, collectionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get content for collection %d: %w", collectionID, err)
	}
	defer rows.Close()

	var contents []*models.Content
	for rows.Next() {
		content := &models.Content{}
		err := rows.Scan(
			&content.ID, &content.SourceID, &content.Title, &content.Body, &content.ContentHash,
			&content.FilePath, &content.FileSize, &content.ContentType, &content.Metadata,
			&content.IsEmbedded, &content.EmbeddingID, &content.CreatedAt, &content.UpdatedAt,
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

func (s *StoreImpl) ListContentByCollection(ctx context.Context, collectionID int64, limit, offset int, sortBy, sortOrder string) ([]*models.Content, error) {
	baseQuery := `
		SELECT c.id, c.source_id, c.title, c.body, c.content_hash, c.file_path, c.file_size, c.content_type, c.metadata, c.is_embedded, c.embedding_id, c.created_at, c.updated_at
		FROM contents c
		JOIN collection_content cc ON c.id = cc.content_id
		WHERE cc.collection_id = $1`

	args := []interface{}{collectionID}
	argID := 2

	// Sorting - Ensure column names are safe and prefixed correctly if needed
	validSortColumns := map[string]string{
		"id":         "c.id",
		"title":      "c.title",
		"created_at": "c.created_at",
		"updated_at": "c.updated_at",
	}
	dbSortColumn, ok := validSortColumns[sortBy]
	if !ok {
		// Check if the provided sortBy is already prefixed (e.g., "c.created_at")
		if _, prefixedOk := validSortColumns[strings.ToLower(sortBy)]; prefixedOk {
			dbSortColumn = sortBy // Use the prefixed version directly
		} else {
			dbSortColumn = "c.created_at" // Default sort column
		}
	}

	sortOrder = strings.ToUpper(sortOrder)
	if sortOrder != "ASC" && sortOrder != "DESC" {
		sortOrder = "DESC" // Default sort order
	}
	orderByClause := fmt.Sprintf(" ORDER BY %s %s", dbSortColumn, sortOrder)

	// Pagination
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	limitClause := fmt.Sprintf(" LIMIT $%d OFFSET $%d", argID, argID+1)
	args = append(args, limit, offset)

	fullQuery := baseQuery + orderByClause + limitClause

	rows, err := s.db.Query(ctx, fullQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list content by collection query: %w", err)
	}
	defer rows.Close()

	var contents []*models.Content
	for rows.Next() {
		content := &models.Content{}
		err := rows.Scan(
			&content.ID, &content.SourceID, &content.Title, &content.Body, &content.ContentHash,
			&content.FilePath, &content.FileSize, &content.ContentType, &content.Metadata,
			&content.IsEmbedded, &content.EmbeddingID, &content.CreatedAt, &content.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan content row for collection: %w", err)
		}
		contents = append(contents, content)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating content rows for collection: %w", err)
	}

	return contents, nil
}

func (s *StoreImpl) getTagsForContentIDs(ctx context.Context, contentIDs []int64) (map[int64][]*models.Tag, error) {
	query := `
		SELECT ct.content_id, t.id, t.name, t.slug, t.created_at, t.updated_at
		FROM content_tags ct
		JOIN tags t ON ct.tag_id = t.id
		WHERE ct.content_id = ANY($1)`
	rows, err := s.db.Query(ctx, query, contentIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags for content IDs: %w", err)
	}
	defer rows.Close()

	result := make(map[int64][]*models.Tag)
	for rows.Next() {
		var contentID int64
		tag := &models.Tag{}
		err := rows.Scan(
			&contentID, &tag.ID, &tag.Name, &tag.Slug, &tag.CreatedAt, &tag.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tag row: %w", err)
		}
		result[contentID] = append(result[contentID], tag)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tag rows: %w", err)
	}
	return result, nil
}

func stringUpper(s string) string {
	return map[bool]string{true: "ASC", false: "DESC"}[s == "ASC" || s == "asc"]
}

// Ensure StoreImpl satisfies the CollectionStore interface
var _ store.CollectionStore = (*StoreImpl)(nil)

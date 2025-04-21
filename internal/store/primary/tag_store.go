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

// --- Tag Management ---

func (s *StoreImpl) CreateTag(ctx context.Context, tag *models.Tag) error {
	query := `
		INSERT INTO tags (name, slug, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`

	now := time.Now()
	if tag.Slug == "" {
		tag.Slug = strings.ToLower(strings.ReplaceAll(tag.Name, " ", "-"))
	}

	err := s.db.QueryRow(ctx, query,
		tag.Name, tag.Slug, now, now,
	).Scan(&tag.ID, &tag.CreatedAt, &tag.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return fmt.Errorf("tag with name or slug already exists: %w", store.ErrDuplicate)
		}
		return fmt.Errorf("failed to insert tag: %w", err)
	}
	return nil
}

func (s *StoreImpl) GetTag(ctx context.Context, id int64) (*models.Tag, error) {
	query := `SELECT id, name, slug, created_at, updated_at FROM tags WHERE id = $1`
	tag := &models.Tag{}
	err := s.db.QueryRow(ctx, query, id).Scan(
		&tag.ID, &tag.Name, &tag.Slug, &tag.CreatedAt, &tag.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get tag by id %d: %w", id, err)
	}
	return tag, nil
}

func (s *StoreImpl) GetTagBySlug(ctx context.Context, slug string) (*models.Tag, error) {
	query := `SELECT id, name, slug, created_at, updated_at FROM tags WHERE slug = $1`
	tag := &models.Tag{}
	err := s.db.QueryRow(ctx, query, slug).Scan(
		&tag.ID, &tag.Name, &tag.Slug, &tag.CreatedAt, &tag.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get tag by slug '%s': %w", slug, err)
	}
	return tag, nil
}

func (s *StoreImpl) GetOrCreateTagsByName(ctx context.Context, names []string) ([]*models.Tag, error) {
	if len(names) == 0 {
		return []*models.Tag{}, nil
	}

	var tags []*models.Tag
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		// Try to find by name (case-insensitive)
		query := `SELECT id, name, slug, created_at, updated_at FROM tags WHERE LOWER(name) = LOWER($1)`
		tag := &models.Tag{}
		err := s.db.QueryRow(ctx, query, name).Scan(
			&tag.ID, &tag.Name, &tag.Slug, &tag.CreatedAt, &tag.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Not found, create it
				newTag := &models.Tag{
					Name: name,
				}
				if err := s.CreateTag(ctx, newTag); err != nil {
					return nil, fmt.Errorf("failed to create tag '%s': %w", name, err)
				}
				tags = append(tags, newTag)
			} else {
				return nil, fmt.Errorf("failed to get tag '%s': %w", name, err)
			}
		} else {
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

func (s *StoreImpl) ListTags(ctx context.Context, limit, offset int) ([]*models.Tag, error) {
	query := `SELECT id, name, slug, created_at, updated_at FROM tags ORDER BY name ASC LIMIT $1 OFFSET $2`
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	defer rows.Close()

	var tags []*models.Tag
	for rows.Next() {
		tag := &models.Tag{}
		err := rows.Scan(
			&tag.ID, &tag.Name, &tag.Slug, &tag.CreatedAt, &tag.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tag row: %w", err)
		}
		tags = append(tags, tag)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tag rows: %w", err)
	}
	return tags, nil
}

func (s *StoreImpl) AddTagsToContent(ctx context.Context, contentID int64, tagIDs []int64) error {
	if len(tagIDs) == 0 {
		return nil
	}
	for _, tagID := range tagIDs {
		query := `
			INSERT INTO content_tags (content_id, tag_id, created_at)
			VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING`
		_, err := s.db.Exec(ctx, query, contentID, tagID, time.Now())
		if err != nil {
			return fmt.Errorf("failed to add tag %d to content %d: %w", tagID, contentID, err)
		}
	}
	return nil
}

func (s *StoreImpl) RemoveTagFromContent(ctx context.Context, contentID, tagID int64) error {
	query := `DELETE FROM content_tags WHERE content_id = $1 AND tag_id = $2`
	_, err := s.db.Exec(ctx, query, contentID, tagID)
	if err != nil {
		return fmt.Errorf("failed to remove tag %d from content %d: %w", tagID, contentID, err)
	}
	return nil
}

func (s *StoreImpl) GetContentTags(ctx context.Context, contentID int64) ([]*models.Tag, error) {
	query := `
		SELECT t.id, t.name, t.slug, t.created_at, t.updated_at
		FROM tags t
		JOIN content_tags ct ON t.id = ct.tag_id
		WHERE ct.content_id = $1
		ORDER BY t.name ASC`
	rows, err := s.db.Query(ctx, query, contentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags for content %d: %w", contentID, err)
	}
	defer rows.Close()

	var tags []*models.Tag
	for rows.Next() {
		tag := &models.Tag{}
		err := rows.Scan(
			&tag.ID, &tag.Name, &tag.Slug, &tag.CreatedAt, &tag.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tag row: %w", err)
		}
		tags = append(tags, tag)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tag rows: %w", err)
	}
	return tags, nil
}

// GetTagsForContents retrieves tags for multiple content items efficiently.
// Returns a map where keys are content IDs and values are slices of associated tags.
func (s *StoreImpl) GetTagsForContents(ctx context.Context, contentIDs []int64) (map[int64][]*models.Tag, error) {
	if len(contentIDs) == 0 {
		return map[int64][]*models.Tag{}, nil
	}

	query := `
		SELECT ct.content_id, t.id, t.name, t.slug, t.created_at, t.updated_at
		FROM tags t
		JOIN content_tags ct ON t.id = ct.tag_id
		WHERE ct.content_id = ANY($1)
		ORDER BY ct.content_id, t.name ASC` // Order by content_id for easier grouping

	rows, err := s.db.Query(ctx, query, contentIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags for multiple contents: %w", err)
	}
	defer rows.Close()

	tagsByContentID := make(map[int64][]*models.Tag)
	for rows.Next() {
		var contentID int64
		tag := &models.Tag{}
		err := rows.Scan(
			&contentID, &tag.ID, &tag.Name, &tag.Slug, &tag.CreatedAt, &tag.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tag row for multiple contents: %w", err)
		}
		tagsByContentID[contentID] = append(tagsByContentID[contentID], tag)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tag rows for multiple contents: %w", err)
	}

	// Ensure all requested content IDs have an entry in the map, even if empty
	for _, id := range contentIDs {
		if _, exists := tagsByContentID[id]; !exists {
			tagsByContentID[id] = []*models.Tag{}
		}
	}

	return tagsByContentID, nil
}

// Ensure StoreImpl satisfies the TagStore interface
var _ store.TagStore = (*StoreImpl)(nil)

package primary // Ensure package declaration is at the top

import (
	"context"
	"fmt"
	"strings"
	"github.com/jackc/pgx/v5"
	"mimir/internal/models"
)

// Ensure pgx types are recognized as used, even if only implicitly via method calls.
var _ pgx.Rows

// KeywordSearchContent performs a full-text search on content body and title.
// It also filters by tags if provided.
func (s *StoreImpl) KeywordSearchContent(ctx context.Context, query string, filterTags []string) ([]*models.Content, error) {
	baseQuery := `
		SELECT DISTINCT c.id, c.source_id, c.title, c.body, c.content_hash, c.file_path, c.file_size, c.content_type, c.metadata, c.embedding_id, c.is_embedded, c.last_accessed_at, c.modified_at, c.summary, c.created_at, c.updated_at
		FROM contents c`
	var joinClause string
	var whereClauses []string
	args := []interface{}{}
	argID := 1

	// Filter by tags if provided
	if len(filterTags) > 0 {
		joinClause = ` JOIN content_tags ct ON c.id = ct.content_id JOIN tags t ON ct.tag_id = t.id`
		tagPlaceholders := make([]string, len(filterTags))
		for i, tag := range filterTags {
			tagPlaceholders[i] = fmt.Sprintf("$%d", argID)
			args = append(args, tag)
			argID++
		}
		whereClauses = append(whereClauses, fmt.Sprintf("t.slug IN (%s)", strings.Join(tagPlaceholders, ",")))
	}

	// Add full-text search condition
	whereClauses = append(whereClauses, fmt.Sprintf("(to_tsvector('english', c.title) @@ plainto_tsquery('english', $%d) OR to_tsvector('english', c.body) @@ plainto_tsquery('english', $%d))", argID, argID))
	args = append(args, query)
	argID++

	finalQuery := baseQuery + joinClause + " WHERE " + strings.Join(whereClauses, " AND ") + " ORDER BY c.created_at DESC" // Add default ordering

	rows, err := s.db.Query(ctx, finalQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query content for keyword search: %w", err)
	}
	defer rows.Close()

	var contents []*models.Content
	for rows.Next() {
		content := &models.Content{}
		// Use the existing scanContent helper function (assuming it takes pgx.Rows)
		if err := scanContent(rows, content); err != nil { // Use the package-level scanContent helper
			return nil, fmt.Errorf("failed to scan content row during keyword search: %w", err)
		}
		contents = append(contents, content)
	}
	return contents, rows.Err()
}

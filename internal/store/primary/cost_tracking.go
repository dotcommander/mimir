package primary

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5" // Import pgx
	"mimir/internal/models"
	// "mimir/internal/store/primary/sqlc" // Import generated sqlc code - Not used in this file
)

// Remove unused CostTrackingStore struct definition
// type CostTrackingStore struct {
// 	db *pgxpool.Pool // Use pgxpool.Pool
// }

// RecordUsage inserts a new AI usage log entry.
func (s *StoreImpl) RecordUsage(ctx context.Context, log *models.AIUsageLog) error { // Implement on StoreImpl
	query := `
		INSERT INTO ai_usage_logs (
			timestamp, provider_name, service_type, model_name,
			input_tokens, output_tokens, cost,
			related_content_id, related_job_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	now := time.Now()
	if log.Timestamp.IsZero() {
		log.Timestamp = now
	}
	err := s.db.QueryRow(ctx, query,
		log.Timestamp,
		log.ProviderName,
		log.ServiceType,
		log.ModelName,
		log.InputTokens,
		log.OutputTokens,
		log.Cost,
		log.RelatedContentID,
		log.RelatedJobID,
	).Scan(&log.ID)
	if err != nil {
		return fmt.Errorf("failed to insert ai_usage_log: %w", err)
	}
	return nil
}

// ListUsage returns a list of AI usage logs, optionally filtered.
func (s *StoreImpl) ListUsage(ctx context.Context, limit, offset int) ([]*models.AIUsageLog, error) { // Implement on StoreImpl
	query := `
		SELECT id, timestamp, provider_name, service_type, model_name,
		       input_tokens, output_tokens, cost, related_content_id, related_job_id
		FROM ai_usage_logs
		ORDER BY timestamp DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := s.db.Query(ctx, query, limit, offset) // Use db.Query
	if err != nil { // Use s.db
		return nil, fmt.Errorf("failed to query ai_usage_logs: %w", err)
	}
	defer rows.Close()

	// Use pgx.CollectRows with explicit type parameter
	// Use pgx.CollectRows with explicit type parameter and correct function signature
	logs, err := pgx.CollectRows[*models.AIUsageLog](rows, func(row pgx.CollectableRow) (*models.AIUsageLog, error) {
		var log models.AIUsageLog
		// Scan all the selected columns into the struct fields
		err := row.Scan(

			&log.ID,
			&log.Timestamp,
			&log.ProviderName,
			&log.ServiceType,
			&log.ModelName,
			&log.InputTokens,
			&log.OutputTokens,
			&log.Cost,
			&log.RelatedContentID,
			&log.RelatedJobID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ai_usage_log: %w", err)
		}
		return &log, nil
	})

	return logs, err // Return collected logs and any error from CollectRows
}

// GetUsageSummary returns the total cost and token usage.
func (s *StoreImpl) GetUsageSummary(ctx context.Context) (totalCost float64, totalInputTokens, totalOutputTokens int64, err error) { // Method receiver should be StoreImpl
	query := `
		SELECT
			COALESCE(SUM(cost),0),
			COALESCE(SUM(input_tokens),0),
			COALESCE(SUM(output_tokens),0)
		FROM ai_usage_logs
	`
	err = s.db.QueryRow(ctx, query).Scan(&totalCost, &totalInputTokens, &totalOutputTokens)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to summarize ai_usage_logs: %w", err)
	}
	return totalCost, totalInputTokens, totalOutputTokens, nil
}

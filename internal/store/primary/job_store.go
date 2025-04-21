package primary

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"mimir/internal/models"
	"mimir/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// --- Job Store Implementation ---

// RecordJobEnqueue inserts a record into the background_jobs table.
func (s *StoreImpl) RecordJobEnqueue(ctx context.Context, params store.JobRecordParams) error {
	query := `
		INSERT INTO background_jobs (job_id, task_type, payload, queue, status, related_entity_type, related_entity_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (job_id) DO NOTHING -- Avoid errors if the same job is somehow recorded twice
		RETURNING id` // Return ID to confirm insertion

	now := time.Now()
	var insertedID int64 // Variable to scan the returned ID into

	// Handle potential nil payload
	var payloadJSON json.RawMessage
	if params.Payload != nil {
		payloadJSON = json.RawMessage(params.Payload)
	} else {
		payloadJSON = json.RawMessage("{}") // Default to empty JSON object if payload is nil
	}

	// Handle optional related entity ID (use sql.NullInt64 for nullable BIGINT)
	var relatedID sql.NullInt64
	if params.RelatedEntityID != 0 {
		relatedID = sql.NullInt64{Int64: params.RelatedEntityID, Valid: true}
	}

	err := s.db.QueryRow(ctx, query,
		params.JobID,
		params.TaskType,
		payloadJSON,
		params.Queue,
		params.Status, // Should be "enqueued" here
		params.RelatedEntityType,
		relatedID,
		now,
		now,
	).Scan(&insertedID)

	if err != nil {
		// If ON CONFLICT DO NOTHING resulted in no row inserted, Scan returns pgx.ErrNoRows.
		// This is not an actual error in our case, as the job was already recorded.
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("DEBUG: Job %s already recorded, skipping insertion.", params.JobID)
			return nil
		}
		// Log the actual error for debugging
		log.Printf("ERROR: Failed to record job enqueue event for JobID %s: %v. Query: %s, Args: %v", params.JobID, err, query, []interface{}{params.JobID, params.TaskType, payloadJSON, params.Queue, params.Status, params.RelatedEntityType, relatedID, now, now})
		return fmt.Errorf("failed to record job enqueue event for JobID %s: %w", params.JobID, err)
	}

	log.Printf("DEBUG: Recorded job enqueue event for JobID %s with DB ID %d", params.JobID, insertedID)
	return nil
}

// UpdateJobStatus updates the status of a job given its Asynq Task UUID.
func (s *StoreImpl) UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status string) error {
	query := `UPDATE background_jobs SET status = $1, updated_at = $2 WHERE job_id = $3`
	now := time.Now()
	cmdTag, err := s.db.Exec(ctx, query, status, now, jobID)
	if err != nil {
		return fmt.Errorf("failed to update job status for job %s: %w", jobID, err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("job %s not found to update status: %w", jobID, store.ErrNotFound)
	}
	return nil
}

// RecordBatchAPIInfo adds the OpenAI Batch API Job ID and Input File ID to an existing job record.
func (s *StoreImpl) RecordBatchAPIInfo(ctx context.Context, jobID uuid.UUID, batchJobID, inputFileID string) error {
	query := `UPDATE background_jobs SET batch_api_job_id = $1, batch_input_file_id = $2, updated_at = $3 WHERE job_id = $4`
	now := time.Now()
	cmdTag, err := s.db.Exec(ctx, query, batchJobID, inputFileID, now, jobID)
	if err != nil {
		return fmt.Errorf("failed to record batch info for job %s: %w", jobID, err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("job %s not found to record batch info: %w", jobID, store.ErrNotFound)
	}
	return nil
}

// UpdateJobStatusAndOutput updates the status and output file ID based on the Batch API Job ID.
func (s *StoreImpl) UpdateJobStatusAndOutput(ctx context.Context, batchJobID, status, outputFileID string) error {
	query := `UPDATE background_jobs SET status = $1, batch_output_file_id = $2, updated_at = $3 WHERE batch_api_job_id = $4`
	now := time.Now()
	// Handle potentially empty outputFileID
	var outputID sql.NullString
	if outputFileID != "" {
		outputID = sql.NullString{String: outputFileID, Valid: true}
	}

	cmdTag, err := s.db.Exec(ctx, query, status, outputID, now, batchJobID)
	if err != nil {
		return fmt.Errorf("failed to update job status/output for batch job %s: %w", batchJobID, err)
	}
	if cmdTag.RowsAffected() == 0 {
		// This might happen if the batch job ID wasn't recorded correctly initially
		log.Printf("WARN: No job found with batch_api_job_id %s to update status/output", batchJobID)
		return store.ErrNotFound // Or return nil depending on desired behavior
	}
	return nil
}

// GetJobByBatchID retrieves job details using the OpenAI Batch API Job ID.
func (s *StoreImpl) GetJobByBatchID(ctx context.Context, batchJobID string) (*models.BackgroundJob, error) {
	// Note: Assumes models.BackgroundJob struct exists and mirrors the table
	query := `SELECT id, job_id, task_type, payload, queue, status, related_entity_type, related_entity_id, batch_api_job_id, batch_input_file_id, batch_output_file_id, job_data, created_at, updated_at
              FROM background_jobs WHERE batch_api_job_id = $1` // Added job_data
	job := &models.BackgroundJob{}
	err := s.db.QueryRow(ctx, query, batchJobID).Scan(
		&job.ID, &job.JobID, &job.TaskType, &job.Payload, &job.Queue, &job.Status,
		&job.RelatedEntityType, &job.RelatedEntityID, &job.BatchAPIJobID,
		&job.BatchInputFileID, &job.BatchOutputFileID, &job.JobData, // Scan job_data
		&job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get job by batch id %s: %w", batchJobID, err)
	}
	return job, nil
}

// UpdateJobData updates the job_data field for a specific job.
func (s *StoreImpl) UpdateJobData(ctx context.Context, jobID uuid.UUID, jobData json.RawMessage) error {
	query := `UPDATE background_jobs SET job_data = $1, updated_at = $2 WHERE job_id = $3`
	now := time.Now()
	cmdTag, err := s.db.Exec(ctx, query, jobData, now, jobID)
	if err != nil {
		return fmt.Errorf("failed to update job data for job %s: %w", jobID, err)
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("job %s not found to update job data: %w", jobID, store.ErrNotFound)
	}
	return nil
}

// ListBatchJobs retrieves background jobs that have an associated Batch API Job ID.
func (s *StoreImpl) ListBatchJobs(ctx context.Context, limit, offset int) ([]*models.BackgroundJob, error) {
	query := `
		SELECT id, job_id, task_type, payload, queue, status, related_entity_type, related_entity_id,
		       batch_api_job_id, batch_input_file_id, batch_output_file_id, job_data, created_at, updated_at
		FROM background_jobs
		WHERE batch_api_job_id IS NOT NULL AND batch_api_job_id != ''
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := s.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query batch jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*models.BackgroundJob
	for rows.Next() {
		job := &models.BackgroundJob{}
		err := rows.Scan(
			&job.ID, &job.JobID, &job.TaskType, &job.Payload, &job.Queue, &job.Status,
			&job.RelatedEntityType, &job.RelatedEntityID, &job.BatchAPIJobID,
			&job.BatchInputFileID, &job.BatchOutputFileID, &job.JobData,
			&job.CreatedAt, &job.UpdatedAt,
		)
		if err != nil {
			// Log the specific error during scan
			log.Printf("ERROR: Failed to scan batch job row: %v", err)
			// Continue processing other rows, but maybe return the error later?
			// For now, let's collect what we can and return the error if any occurred.
			return jobs, fmt.Errorf("failed to scan batch job row: %w", err)
		}
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return jobs, fmt.Errorf("error iterating batch job rows: %w", err)
	}

	return jobs, nil
}

// Ensure StoreImpl satisfies the JobStore interface
var _ store.JobStore = (*StoreImpl)(nil)

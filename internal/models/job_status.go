package models

/*
Job and Task status/type constants for use throughout the codebase.
Centralizing these avoids magic strings and improves maintainability.
*/

// Job status constants
const (
	JobStatusEnqueued        = "enqueued"
	JobStatusProcessingBatch = "processing_batch"
	JobStatusCompleted       = "completed"
	JobStatusFailed          = "failed"
	JobStatusPending         = "pending"
	JobStatusRunning         = "running"
	JobStatusCancelled       = "cancelled"
	JobStatusRetrying        = "retrying"
	JobStatusTimedOut        = "timed_out"
)

// Task type constants
const (
	TaskTypeEmbedding      = "embedding"
	TaskTypeEmbeddingCheck = "embedding_check"
	TaskTypeSummarization  = "summarization"
	TaskTypeCategorization = "categorization"
	TaskTypeCostTracking   = "cost_tracking"
)

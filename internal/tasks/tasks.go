package tasks

// Defines constants for task types used in Asynq.

const (
	// TypeEmbeddingJob is the task type for generating content embeddings.
	TypeEmbeddingJob = "embedding:generate" // Task to initiate batch job
	// TypeEmbeddingCheckBatch is the task type for checking batch status.
	TypeEmbeddingCheckBatch = "embedding:check_batch" // Task to check batch status

	// TypeSummarizationJob is the task type for generating content summaries.
	TypeSummarizationJob = "summarization:generate"
)

// Add other task type constants here if needed in the future.

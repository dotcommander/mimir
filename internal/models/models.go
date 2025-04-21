package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

// AIUsageLog represents a record of AI API usage for cost tracking.
type AIUsageLog struct {
	ID              int64      `db:"id"`
	Timestamp       time.Time  `db:"timestamp"`
	ProviderName    string     `db:"provider_name"`
	ServiceType     string     `db:"service_type"` // e.g., "embedding", "categorization"
	ModelName       string     `db:"model_name"`
	InputTokens     int        `db:"input_tokens"`
	OutputTokens    int        `db:"output_tokens"`
	Cost            float64    `db:"cost"`
	RelatedContentID *int64    `db:"related_content_id"` // nullable
	RelatedJobID    *uuid.UUID `db:"related_job_id"`     // nullable UUID
}


type Source struct {
	ID          int64     `db:"id"`
	Name        string    `db:"name"`
	Description *string   `db:"description"`
	URL         *string   `db:"url"`
	SourceType  string    `db:"source_type"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type Content struct {
	ID             int64           `db:"id"`
	SourceID       int64           `db:"source_id"`
	Title          string          `db:"title"`
	Body           string          `db:"body"`
	ContentHash    string          `db:"content_hash"`
	FilePath       *string         `db:"file_path"`
	FileSize       *int64          `db:"file_size"`
	ContentType    string          `db:"content_type"`
	Metadata       json.RawMessage `db:"metadata"`
	EmbeddingID    *uuid.UUID      `db:"embedding_id"`
	IsEmbedded     bool            `db:"is_embedded"`
	LastAccessedAt *time.Time      `db:"last_accessed_at"`
	ModifiedAt     *time.Time      `db:"modified_at"` // File modification time (nullable)
	Summary        *string         `db:"summary"`     // Added for summarization
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
}

type Tag struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	Slug      string    `db:"slug"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type Collection struct {
	ID          int64     `db:"id"`
	Name        string    `db:"name"`
	Description *string   `db:"description"`
	IsPinned    bool      `db:"is_pinned"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type SearchQuery struct {
	ID           int64     `db:"id"`
	Query        string    `db:"query"`
	ResultsCount int       `db:"results_count"`
	ExecutedAt   time.Time `db:"executed_at"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

type SearchResult struct {
	ID             int64     `db:"id"`
	SearchQueryID  int64     `db:"search_query_id"`
	ContentID      int64     `db:"content_id"`
	RelevanceScore float64   `db:"relevance_score"`
	Rank           int       `db:"rank"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

type EmbeddingEntry struct {
	ID        uuid.UUID       `db:"id"`
	ContentID int64           `db:"content_id"`
	ChunkText string          `db:"chunk_text"` // Add field for chunk text
	Vector    pgvector.Vector `db:"vector"`     // Renamed field to match DB column 'vector'
	Metadata  json.RawMessage `db:"metadata"`
	CreatedAt time.Time       `db:"created_at"`
}

// BackgroundJob mirrors the background_jobs table schema.
type BackgroundJob struct {
	ID                int64           `db:"id"`
	JobID             uuid.UUID       `db:"job_id"` // Asynq Task ID
	TaskType          string          `db:"task_type"`
	Payload           json.RawMessage `db:"payload"`
	Queue             string          `db:"queue"`
	Status            string          `db:"status"`
	RelatedEntityType *string         `db:"related_entity_type"`  // Use pointer for NULLable
	RelatedEntityID   *int64          `db:"related_entity_id"`    // Use pointer for NULLable
	BatchAPIJobID     *string         `db:"batch_api_job_id"`     // Use pointer for NULLable
	BatchInputFileID  *string         `db:"batch_input_file_id"`  // Use pointer for NULLable
	BatchOutputFileID *string         `db:"batch_output_file_id"` // Use pointer for NULLable
	JobData           json.RawMessage `db:"job_data"`             // Add field for job data (e.g., chunks)
	// Summary field removed - belongs to Content model
	CreatedAt         time.Time       `db:"created_at"`
	UpdatedAt         time.Time       `db:"updated_at"`
}

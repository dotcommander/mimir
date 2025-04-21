package store

import (
	"context"
	"encoding/json" // Add json import for RawMessage
	"mimir/internal/models"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/pgvector/pgvector-go"
	// "mimir/internal/services" // REMOVED to break import cycle
)

// --- Provider Status (Defined here to break import cycle) ---

type ProviderStatus int

const (
	ProviderStatusUnknown  ProviderStatus = iota // Default zero value
	ProviderStatusActive                         // Provider is operational
	ProviderStatusInactive                       // Provider is temporarily unavailable (e.g., network, rate limit)
	ProviderStatusDisabled                       // Provider is not configured or explicitly disabled
)

// --- Job Client ---

type JobClient interface {
	// Enqueue now includes related entity info for recording purposes
	Enqueue(ctx context.Context, task *asynq.Task, relatedEntityType string, relatedEntityID int64, opts ...asynq.Option) (*asynq.TaskInfo, error)
	EnqueueEmbeddingJob(ctx context.Context, contentID int64) error
	Close() error // Ensure Close is part of the interface
}

// --- Content Store ---

type ContentStore interface {
	CreateContent(ctx context.Context, content *models.Content) error
	GetContent(ctx context.Context, id int64) (*models.Content, error)
	UpdateContent(ctx context.Context, content *models.Content) error
	DeleteContent(ctx context.Context, id int64) error
	ListContent(ctx context.Context, limit, offset int, sortBy, sortOrder string, filterTags []string) ([]*models.Content, error)
	FindContentByHash(ctx context.Context, hash string) (*models.Content, error)
	UpdateContentEmbeddingStatus(ctx context.Context, contentID int64, embeddingID uuid.UUID, isEmbedded bool) error
	CreateContentIfNotExists(ctx context.Context, content *models.Content) (bool, error)
	GetContentsByIDs(ctx context.Context, ids []int64) ([]*models.Content, error)

	Ping(ctx context.Context) error
}

// --- Source Store ---

type SourceStore interface {
	CreateSource(ctx context.Context, source *models.Source) error
	GetSource(ctx context.Context, id int64) (*models.Source, error)
	GetSourceByName(ctx context.Context, name string) (*models.Source, error)
	ListSources(ctx context.Context, limit, offset int) ([]*models.Source, error)
}

// --- Tag Store ---

type TagStore interface {
	CreateTag(ctx context.Context, tag *models.Tag) error
	GetTag(ctx context.Context, id int64) (*models.Tag, error)
	GetTagBySlug(ctx context.Context, slug string) (*models.Tag, error)
	GetOrCreateTagsByName(ctx context.Context, names []string) ([]*models.Tag, error)
	ListTags(ctx context.Context, limit, offset int) ([]*models.Tag, error)
	AddTagsToContent(ctx context.Context, contentID int64, tagIDs []int64) error
	RemoveTagFromContent(ctx context.Context, contentID, tagID int64) error
	GetContentTags(ctx context.Context, contentID int64) ([]*models.Tag, error)
	GetTagsForContents(ctx context.Context, contentIDs []int64) (map[int64][]*models.Tag, error) // Add method for batch tag fetching
}

// --- Collection Store ---

type CollectionStore interface {
	CreateCollection(ctx context.Context, collection *models.Collection) error
	GetCollection(ctx context.Context, id int64) (*models.Collection, error)
	GetCollectionByName(ctx context.Context, name string) (*models.Collection, error)
	ListCollections(ctx context.Context, limit, offset int, pinned *bool) ([]*models.Collection, error)
	UpdateCollection(ctx context.Context, collection *models.Collection) error
	DeleteCollection(ctx context.Context, id int64) error
	AddContentToCollection(ctx context.Context, collectionID, contentID int64) error
	RemoveContentFromCollection(ctx context.Context, collectionID, contentID int64) error
	GetCollectionContent(ctx context.Context, collectionID int64, limit, offset int) ([]*models.Content, error)
	ListContentByCollection(ctx context.Context, collectionID int64, limit, offset int, sortBy, sortOrder string) ([]*models.Content, error)
}

// --- Search History Store ---

type SearchHistoryStore interface {
	RecordSearchQuery(ctx context.Context, query string, resultsCount int) (*models.SearchQuery, error)
	ListSearchQueries(ctx context.Context, limit int) ([]*models.SearchQuery, error)
	RecordSearchResults(ctx context.Context, queryID int64, results []models.SearchResult) error
}

// --- Keyword Search ---

type KeywordSearcher interface {
	KeywordSearchContent(ctx context.Context, query string, filterTags []string) ([]*models.Content, error)
}

// --- Vector Store ---

type VectorStore interface {
	AddEmbedding(ctx context.Context, entry *models.EmbeddingEntry) error
	GetEmbedding(ctx context.Context, id uuid.UUID) (*models.EmbeddingEntry, error)
	DeleteEmbeddingsByContentID(ctx context.Context, contentID int64) error
	SimilaritySearch(ctx context.Context, queryVector pgvector.Vector, k int, filterMetadata map[string]interface{}) ([]models.SearchResult, error)

	Ping(ctx context.Context) error
	Close() error
}

// --- Embedding Service ---

type EmbeddingService interface {
	GenerateEmbedding(ctx context.Context, text string) (pgvector.Vector, error)
	GenerateEmbeddings(ctx context.Context, texts []string) ([]pgvector.Vector, error)
	Dimension() int
	ModelName() string      // Add ModelName method
	Name() string           // Add Name method
	Status() ProviderStatus // Use locally defined ProviderStatus
}

// --- Job Store ---

// JobRecordParams holds parameters for recording a job event.
type JobRecordParams struct {
	JobID             uuid.UUID
	TaskType          string
	Payload           []byte
	Queue             string
	Status            string
	RelatedEntityType string // Optional: e.g., "content"
	RelatedEntityID   int64  // Optional: e.g., content.ID
}

type JobStore interface {
	RecordJobEnqueue(ctx context.Context, params JobRecordParams) error
	UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status string) error                     // Add missing method
	RecordBatchAPIInfo(ctx context.Context, jobID uuid.UUID, batchJobID, inputFileID string) error // Add missing method
	UpdateJobStatusAndOutput(ctx context.Context, batchJobID, status, outputFileID string) error   // Add missing method
	GetJobByBatchID(ctx context.Context, batchJobID string) (*models.BackgroundJob, error)         // Add missing method
	UpdateJobData(ctx context.Context, jobID uuid.UUID, jobData json.RawMessage) error             // Add method to store job data (e.g., chunks)
	ListBatchJobs(ctx context.Context, limit, offset int) ([]*models.BackgroundJob, error)         // Add method to list jobs with batch IDs
}

// --- Cost Tracking Store ---

type CostTrackingStore interface {
	RecordUsage(ctx context.Context, log *models.AIUsageLog) error
	ListUsage(ctx context.Context, limit, offset int) ([]*models.AIUsageLog, error)
	GetUsageSummary(ctx context.Context) (totalCost float64, totalInputTokens, totalOutputTokens int64, err error)
}

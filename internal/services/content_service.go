package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"mimir/internal/config"
	"mimir/internal/inputprocessor"
	"mimir/internal/models"
	"mimir/internal/tasks" // Ensure this import is uncommented
	"mimir/internal/store"
)

// ContentInputResult holds extracted content details
// Note: Field names and types updated to match PrepareContentInput assignments.
type ContentInputResult struct {
	Body          string // Extracted text content (was Content)
	ContentType   string // e.g., "text/plain", "application/pdf"
	InputType     string // "File", "URL", "RawString", "Directory"
	OriginalInput string
	FilePath      *string    // Pointer: Absolute path if InputType is File or Directory
	FileSize      *int64     // Pointer: Size if InputType is File
	URL           *string    // Pointer: URL if InputType is URL
	Mtime         *time.Time // Pointer: File modification time if available
	Metadata      map[string]interface{}
}

type ContentResultItem struct {
	Content models.Content
	Tags    []*models.Tag
}

type CollectionResultItem struct {
	Collection *models.Collection
	Tags       []*models.Tag
}

type KeywordResultItem struct {
	Content *models.Content
	Score   float64
}

type ContentService struct {
	contents       store.ContentStore
	tags           store.TagStore
	jobs           store.JobClient
	ss             *SourceService
	processor      inputprocessor.Processor
	summaryService SummaryService
	taggingService TaggingService

	deps ContentServiceDeps // Store all dependencies for later use
}

// Removed duplicate AddContentParams struct definition

type ListContentParams struct {
	Limit      int
	Offset     int
	SortBy     string
	SortOrder  string
	FilterTags []string
}

// Update constructor signature to accept inputprocessor.Processor
type ContentServiceDeps struct {
	ContentStore          store.ContentStore
	TagStore              store.TagStore
	JobClient             store.JobClient
	SourceService         *SourceService
	Processor             inputprocessor.Processor
	SummaryService        SummaryService
	TaggingService        TaggingService
	CategorizationService *CategorizationService // Add this line
	Config                *config.Config         // Add config reference
}

func NewContentService(deps ContentServiceDeps) *ContentService {
	return &ContentService{
		contents:       deps.ContentStore,
		tags:           deps.TagStore,
		jobs:           deps.JobClient,
		ss:             deps.SourceService,
		processor:      deps.Processor,
		summaryService: deps.SummaryService,
		taggingService: deps.TaggingService,
		deps:           deps,
	}
}

// REMOVE the PrepareContentInput method entirely from ContentService
// func (cs *ContentService) PrepareContentInput(...) { ... }

// Update AddContent signature and logic
// Change params to accept RawInput instead of processed fields
type AddContentParams struct {
	SourceName string
	Title      string
	RawInput   string // Input string (file path, URL, or raw text)
	SourceType string // Type of the source (e.g., "cli", "web")
}

func (cs *ContentService) AddContent(ctx context.Context, params AddContentParams) (*models.Content, bool, error) {
	inputResult, err := cs.processInput(ctx, params.RawInput)
	if err != nil {
		return nil, false, err
	}

	source, err := cs.getOrCreateSource(ctx, params.SourceName, params.SourceType, inputResult)
	if err != nil {
		return nil, false, err
	}

	content := cs.buildContentModel(source.ID, params.Title, inputResult)

	existed, err := cs.contents.CreateContentIfNotExists(ctx, content)
	if err != nil {
		return nil, false, fmt.Errorf("create content: %w", err)
	}

	if !existed {
		cs.enqueueEmbeddingJobIfPossible(ctx, content)
		// Auto-tagging: If enabled, apply tags using CategorizationService
		if cs.deps.Config != nil && cs.deps.Config.Categorization.AutoApplyTags && cs.deps.CategorizationService != nil {
			res, err := cs.deps.CategorizationService.CategorizeContent(ctx, content.Title, content.Body, nil)
			if err == nil && res != nil && len(res.Tags) > 0 {
				tagObjs, err := cs.tags.GetOrCreateTagsByName(ctx, res.Tags)
				if err == nil && len(tagObjs) > 0 {
					tagIDs := make([]int64, len(tagObjs))
					for i, t := range tagObjs {
						tagIDs[i] = t.ID
					}
					_ = cs.tags.AddTagsToContent(ctx, content.ID, tagIDs)
				} else if err != nil {
					log.Printf("WARN: Failed to get/create tags (%v) for content %d: %v", res.Tags, content.ID, err)
				}
			} else if err != nil {
				log.Printf("WARN: Failed to categorize content %d: %v", content.ID, err)
			}
		}
		// Summarization: If enabled, enqueue summarization job asynchronously
		if cs.deps.Config != nil && cs.deps.Config.Summarization.Enabled && cs.jobs != nil {
			// Enqueue summarization job
			// Define the payload structure locally for marshalling.
			// The worker will define and unmarshal its own expected payload.
			type summarizationPayload struct {
				ContentID int64 `json:"ContentID"`
			}
			payload := summarizationPayload{ContentID: content.ID}
			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				log.Printf("ERROR: Failed to marshal summarization payload for content %d: %v", content.ID, err)
				// Decide if this should prevent adding content or just log
			} else {
				queueName := "default" // Default queue
				if q, ok := cs.deps.Config.Worker.Queues["summarization"]; ok {
					queueName = "summarization" // Use specific queue if defined
					log.Printf("Using 'summarization' queue (priority %d) for summarization job.", q)
				} else if q, ok := cs.deps.Config.Worker.Queues["default"]; ok {
					log.Printf("Using 'default' queue (priority %d) for summarization job.", q)
				}

				task := asynq.NewTask(tasks.TypeSummarizationJob, payloadBytes)
				// Enqueue with appropriate options (e.g., queue name from config)
				_, err := cs.jobs.Enqueue(ctx, task,
					"content", // relatedEntityType
					content.ID, // relatedEntityID
					asynq.Queue(queueName),
					asynq.MaxRetry(3),             // Example retry count
					asynq.Timeout(5*time.Minute), // Example timeout
				)
				if err != nil {
					log.Printf("ERROR: Failed to enqueue summarization job for content %d: %v", content.ID, err)
					// Potentially return error or just log
				} else {
					log.Printf("Successfully enqueued summarization job for content %d", content.ID)
				}
			}
		} else if cs.deps.Config != nil && cs.deps.Config.Summarization.Enabled && cs.jobs == nil {
			log.Printf("WARN: Summarization enabled but JobClient (cs.jobs) is nil. Cannot enqueue summarization job for content %d.", content.ID)
		}
	}

	log.Printf("AddContent: content_id=%d, existed=%v, title=%q, source=%q", content.ID, existed, content.Title, params.SourceName)

	return content, existed, nil
}

func (cs *ContentService) summarizeAndTagContent(ctx context.Context, content *models.Content) {
	// Summarization (no-op for now)
	// Pass contentID and an empty jobID string for now
	// TODO: Determine if a meaningful jobID can be passed here for cost tracking context
	jobIDPlaceholder := "" // Or generate a UUID?
	summary, err := cs.summaryService.Summarize(ctx, content.Body, content.ID, jobIDPlaceholder)
	if err == nil && summary != "" {
		content.Summary = &summary
		_ = cs.contents.UpdateContent(ctx, content)
	}

	// Tagging (no-op for now)
	tags, err := cs.taggingService.SuggestTags(ctx, content.Body)
	if err == nil && len(tags) > 0 {
		tagObjs, err := cs.tags.GetOrCreateTagsByName(ctx, tags)
		if err == nil && len(tagObjs) > 0 {
			tagIDs := make([]int64, len(tagObjs))
			for i, t := range tagObjs {
				tagIDs[i] = t.ID
			}
			_ = cs.tags.AddTagsToContent(ctx, content.ID, tagIDs)
		}
	}
}

// processInput handles input processing and error wrapping.
func (cs *ContentService) processInput(ctx context.Context, rawInput string) (inputprocessor.Result, error) {
	inputResult, err := cs.processor.Process(ctx, rawInput)
	if err != nil {
		return inputprocessor.Result{}, fmt.Errorf("failed to process input '%s': %w", rawInput, err)
	}
	return inputResult, nil
}

// getOrCreateSource wraps source creation and error handling.
func (cs *ContentService) getOrCreateSource(ctx context.Context, sourceName, sourceType string, inputResult inputprocessor.Result) (*models.Source, error) {
	sourceURL := getSourceURLFromInputResult(inputResult)
	source, err := cs.ss.GetOrCreateSource(ctx, GetOrCreateSourceParams{
		Name: sourceName,
		Type: sourceType,
		Desc: nil,
		URL:  sourceURL,
	})
	if err != nil {
		return nil, fmt.Errorf("get/create source '%s': %w", sourceName, err)
	}
	if source == nil {
		return nil, fmt.Errorf("failed to get or create source '%s' (returned nil source object)", sourceName)
	}
	return source, nil
}

// buildContentModel constructs a models.Content from input processor results.
func (cs *ContentService) buildContentModel(sourceID int64, title string, inputResult inputprocessor.Result) *models.Content {
	content := &models.Content{
		SourceID:    sourceID,
		Title:       title,
		Body:        inputResult.Body,
		ContentType: inputResult.ContentType,
		FilePath:    inputResult.FilePath,
		FileSize:    inputResult.FileSize,
		ModifiedAt:  inputResult.Mtime, // Map Mtime here
	}
	return content
}

// getSourceURLFromInputResult extracts the source URL from the input processor result. // Keep this helper
func getSourceURLFromInputResult(inputResult inputprocessor.Result) *string {
	if inputResult.URL != nil {
		return inputResult.URL
	}
	if inputResult.FilePath != nil {
		// Optionally convert file path to file:// URL here if desired.
	}
	return nil
}

// buildContentModelFromInput constructs a models.Content from input processor results.
func buildContentModelFromInput(sourceID int64, title string, inputResult inputprocessor.Result) *models.Content { // Keep this helper
	content := &models.Content{
		SourceID:    sourceID,
		Title:       title,
		Body:        inputResult.Body,
		ContentType: inputResult.ContentType,
		FilePath:    inputResult.FilePath,
		FileSize:    inputResult.FileSize,
		ModifiedAt:  inputResult.Mtime, // Map Mtime here
	}
	return content
}

// enqueueEmbeddingJobIfPossible enqueues an embedding job if the job client is available.
func (cs *ContentService) enqueueEmbeddingJobIfPossible(ctx context.Context, content *models.Content) {
	if cs.jobs != nil {
		log.Printf("DEBUG: About to call EnqueueEmbeddingJob on cs.jobs (type: %T, value: %p)", cs.jobs, cs.jobs)
		err := cs.jobs.EnqueueEmbeddingJob(ctx, content.ID)
		if err != nil {
			log.Printf("ERROR: EnqueueEmbeddingJob failed for content %d: %v", content.ID, err)
		} else {
			log.Printf("DEBUG: EnqueueEmbeddingJob call appears to have succeeded for content %d", content.ID)
		}
	} else {
		log.Printf("WARN: cs.jobs is nil, skipping embedding job enqueue for content %d", content.ID)
	}
}

func (cs *ContentService) ListContent(ctx context.Context, params ListContentParams) ([]ContentResultItem, error) {
	contents, err := cs.contents.ListContent(ctx, params.Limit, params.Offset, params.SortBy, params.SortOrder, params.FilterTags)
	if err != nil {
		return nil, fmt.Errorf("list content: %w", err)
	}

	return cs.attachTagsToContents(ctx, contents)
}

// attachTagsToContents attaches tags to each content item and returns ContentResultItem slices.
func (cs *ContentService) attachTagsToContents(ctx context.Context, contents []*models.Content) ([]ContentResultItem, error) {
	result := make([]ContentResultItem, len(contents))
	for i, c := range contents {
		tags, err := cs.tags.GetContentTags(ctx, c.ID)
		if err != nil {
			log.Printf("WARN: fetch tags for content %d: %v", c.ID, err)
			tags = []*models.Tag{}
		}
		result[i] = ContentResultItem{Content: *c, Tags: tags}
	}
	return result, nil
}

func (cs *ContentService) DeleteContent(ctx context.Context, contentID int64, vs store.VectorStore) error {
	if err := cs.deleteEmbeddingsIfPresent(ctx, contentID, vs); err != nil {
		return fmt.Errorf("DeleteContent: %w", err)
	}

	if err := cs.deleteContentFromStore(ctx, contentID); err != nil {
		return fmt.Errorf("DeleteContent: %w", err)
	}

	return nil
}

// deleteEmbeddingsIfPresent deletes embeddings for the content if a VectorStore is provided.
func (cs *ContentService) deleteEmbeddingsIfPresent(ctx context.Context, contentID int64, vs store.VectorStore) error {
	if vs != nil {
		err := vs.DeleteEmbeddingsByContentID(ctx, contentID)
		if err != nil {
			return fmt.Errorf("deleteEmbeddingsIfPresent: %w", err)
		}
	}
	return nil
}

// deleteContentFromStore deletes the content from the ContentStore.
func (cs *ContentService) deleteContentFromStore(ctx context.Context, contentID int64) error {
	err := cs.contents.DeleteContent(ctx, contentID)
	if err != nil {
		return fmt.Errorf("deleteContentFromStore: %w", err)
	}
	return nil
}

// GetContent retrieves a single content item by its ID.
func (cs *ContentService) GetContent(ctx context.Context, id int64) (*models.Content, error) {
	content, err := cs.contents.GetContent(ctx, id)
	if err != nil {
		// Wrap the error for context, potentially checking for store.ErrNotFound
		return nil, fmt.Errorf("GetContent: failed to get content with ID %d from store: %w", id, err)
	}
	return content, nil
}

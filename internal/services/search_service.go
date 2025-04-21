package services

import (
	"context" // Add context import
	"fmt"
	"log"

	"mimir/internal/models"
	"mimir/internal/store"
)
import "errors" // Add errors import // Keep this one

// SearchResultItem represents a single search result, potentially including chunk details.
type SearchResultItem struct {
	Content       *models.Content
	Score         float64
	// Removed ChunkText and ChunkMetadata as they are not available
	// from the current vector.SimilaritySearch return type.
	// ChunkText     string                 // Text of the specific chunk that matched
	// ChunkMetadata map[string]interface{} // Metadata associated with the matched chunk
}

type SearchService struct {
	contentStore    store.ContentStore
	keywordSearcher store.KeywordSearcher
	vector          store.VectorStore
	embedding       store.EmbeddingService
	searchHistory   store.SearchHistoryStore
}

func NewSearchService(cs store.ContentStore, ks store.KeywordSearcher, vs store.VectorStore, es store.EmbeddingService, sh store.SearchHistoryStore) *SearchService {
	return &SearchService{
		contentStore:    cs,
		keywordSearcher: ks,
		vector:          vs,
		embedding:       es,
		searchHistory:   sh,
	}
}

// --- Parameter Structs ---

type KeywordSearchParams struct {
	Query      string
	FilterTags []string
	Limit      int
	Offset     int
}

type SemanticSearchParams struct {
	Query      string
	Limit      int
	FilterTags []string
}

type RelatedContentParams struct {
	SourceContentID int64
	Limit           int
	FilterTags      []string
}

// --- Service Methods ---

func (s *SearchService) KeywordSearch(ctx context.Context, params KeywordSearchParams) ([]KeywordResultItem, error) {
	if s.keywordSearcher == nil {
		return nil, fmt.Errorf("keyword searcher is not initialized")
	}

	// Record the search query attempt
	searchQueryRecord, errRecord := s.searchHistory.RecordSearchQuery(ctx, params.Query, 0) // Record with 0 results initially
	if errRecord != nil {
		// Log the error but don't fail the search itself
		log.Printf("WARN: Failed to record keyword search query '%s': %v", params.Query, errRecord)
	}

	if params.Limit > 0 || params.Offset > 0 {
		log.Printf("WARN: KeywordSearch Limit/Offset parameters are currently ignored.")
	}

	results, err := s.keywordSearcher.KeywordSearchContent(ctx, params.Query, params.FilterTags)
	if err != nil {
		return nil, fmt.Errorf("keyword search failed: %w", err)
	}

	serviceResults := make([]KeywordResultItem, len(results))
	for i, storeResult := range results {
		if storeResult != nil {
			serviceResults[i] = KeywordResultItem{
				Content: storeResult,
				Score:   0.0,
			}
		} else {
			log.Printf("WARN: KeywordSearch store result or its content was nil at index %d", i)
		}
	}

	// If recording was successful, update the count and record results
	if errRecord == nil && searchQueryRecord != nil {
		searchQueryRecord.ResultsCount = len(results) // Update count based on actual results

		// Prepare results for recording
		recordedResults := make([]models.SearchResult, len(results))
		for i, res := range results {
			if res != nil {
				recordedResults[i] = models.SearchResult{
					ContentID:      res.ID,
					RelevanceScore: 0.0,
					Rank:           i + 1,
				}
			}
		}

		errUpdate := s.searchHistory.RecordSearchResults(ctx, searchQueryRecord.ID, recordedResults)
		if errUpdate != nil {
			log.Printf("WARN: Failed to record keyword search results for query ID %d: %v", searchQueryRecord.ID, errUpdate)
		}
	}

	return serviceResults, nil
}

// SemanticSearch performs vector similarity search based on the query text.
func (s *SearchService) SemanticSearch(ctx context.Context, params SemanticSearchParams) ([]SearchResultItem, error) {
	if s.vector == nil {
		return nil, fmt.Errorf("vector store is not initialized")
	}
	if s.embedding == nil {
		return nil, fmt.Errorf("embedding service is not initialized")
	}
	if params.Limit <= 0 {
		params.Limit = 10
	}

	// Record the search query attempt
	searchQueryRecord, errRecord := s.searchHistory.RecordSearchQuery(ctx, params.Query, 0) // Record with 0 results initially
	if errRecord != nil {
		// Log the error but don't fail the search itself
		log.Printf("WARN: Failed to record semantic search query '%s': %v", params.Query, errRecord)
	}

	queryVector, err := s.embedding.GenerateEmbedding(ctx, params.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	filterMetadata := make(map[string]interface{})
	if len(params.FilterTags) > 0 {
		log.Printf("WARN: SemanticSearch tag filtering is not yet implemented in the vector query.")
	}

	// Use the modified SimilaritySearch which returns more details
	vectorResults, err := s.vector.SimilaritySearch(ctx, queryVector, params.Limit, filterMetadata) // Returns []vector.VectorSearchResultItem
	if err != nil {
		return nil, fmt.Errorf("vector similarity search failed: %w", err)
	}

	contentIDs := make([]int64, len(vectorResults))
	// Keep track of vector results by content ID for easier lookup later
	scoresMap := make(map[int64]float64)
	for i, res := range vectorResults {
		contentIDs[i] = res.ContentID
		scoresMap[res.ContentID] = res.RelevanceScore
	}

	if len(contentIDs) == 0 {
		return []SearchResultItem{}, nil
	}

	contents, err := s.contentStore.GetContentsByIDs(ctx, contentIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch content details for search results: %w", err)
	}

	// Create a map for quick lookup of content by ID
	contentMap := make(map[int64]*models.Content, len(contents))
	for _, c := range contents {
		if c != nil {
			contentMap[c.ID] = c
		}
	}

	results := make([]SearchResultItem, 0, len(contents))
	// Iterate through the vector results to build the final SearchResultItems
	for _, vecRes := range vectorResults {
		content, contentFound := contentMap[vecRes.ContentID]
		if !contentFound {
			log.Printf("WARN: Content %d found in vector search but not retrieved from primary store.", vecRes.ContentID)
			continue
		}

		results = append(results, SearchResultItem{
			Content: content,
			Score:   vecRes.RelevanceScore,
			// ChunkText and ChunkMetadata removed
			// ChunkText: vecRes.ChunkText,
			// ChunkMetadata: chunkMeta,
		})
	}
	// If recording was successful, update the count and record results
	if errRecord == nil && searchQueryRecord != nil {
		searchQueryRecord.ResultsCount = len(results) // Update count based on actual results

		// Prepare results for recording
		recordedResults := make([]models.SearchResult, len(results))
		for i, res := range results {
			if res.Content != nil {
				recordedResults[i] = models.SearchResult{
					ContentID:      res.Content.ID,
					RelevanceScore: res.Score,
					Rank:           i + 1,
				}
			}
		}

		errUpdate := s.searchHistory.RecordSearchResults(ctx, searchQueryRecord.ID, recordedResults)
		if errUpdate != nil {
			log.Printf("WARN: Failed to record semantic search results for query ID %d: %v", searchQueryRecord.ID, errUpdate)
		}
	}

	return results, nil
}

// ListSearchHistory retrieves recent search queries.
func (s *SearchService) ListSearchHistory(ctx context.Context, limit int) ([]*models.SearchQuery, error) {
	if s.searchHistory == nil {
		return nil, fmt.Errorf("search history store is not initialized")
	}
	queries, err := s.searchHistory.ListSearchQueries(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list search history from store: %w", err)
	}
	return queries, nil
}

// FindRelatedContent finds content semantically similar to a given source content item.
func (s *SearchService) FindRelatedContent(ctx context.Context, params RelatedContentParams) ([]SearchResultItem, error) {
	if s.contentStore == nil {
		return nil, fmt.Errorf("content store is not initialized")
	}
	if s.vector == nil {
		return nil, fmt.Errorf("vector store is not initialized")
	}

	if params.Limit <= 0 {
		params.Limit = 10
	}

	sourceContent, err := s.contentStore.GetContent(ctx, params.SourceContentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("source content with ID %d not found: %w", params.SourceContentID, err)
		}
		return nil, fmt.Errorf("failed to get source content %d: %w", params.SourceContentID, err)
	}

	if !sourceContent.IsEmbedded || sourceContent.EmbeddingID == nil {
		return nil, fmt.Errorf("source content %d has not been embedded, cannot find related content", params.SourceContentID)
	}

	sourceEmbeddingEntry, err := s.vector.GetEmbedding(ctx, *sourceContent.EmbeddingID)
	if err != nil {
		// No longer need to check for ErrOperationNotSupported as pgvector supports GetEmbedding
		if errors.Is(err, store.ErrNotFound) {
			log.Printf("ERROR: Embedding ID %s found in primary store for content %d, but not found in vector store.", sourceContent.EmbeddingID.String(), sourceContent.ID)
			return nil, fmt.Errorf("embedding for source content %d not found in vector store (ID: %s)", params.SourceContentID, sourceContent.EmbeddingID.String())
		}
		return nil, fmt.Errorf("failed to get source embedding (ID: %s) from vector store: %w", sourceContent.EmbeddingID.String(), err)
	}
	sourceVector := sourceEmbeddingEntry.Vector // Changed Embedding to Vector

	filterMetadata := make(map[string]interface{})
	if len(params.FilterTags) > 0 {
		log.Printf("WARN: FindRelatedContent tag filtering is not yet implemented in the vector query.")
	}

	vectorResults, err := s.vector.SimilaritySearch(ctx, sourceVector, params.Limit+1, filterMetadata)
	if err != nil {
		return nil, fmt.Errorf("vector similarity search failed for related content: %w", err)
	}

	contentIDs := make([]int64, 0, len(vectorResults))
	scoresMap := make(map[int64]float64)
	for _, res := range vectorResults {
		if res.ContentID == params.SourceContentID {
			continue
		}
		contentIDs = append(contentIDs, res.ContentID)
		scoresMap[res.ContentID] = res.RelevanceScore
	}

	if len(contentIDs) == 0 {
		return []SearchResultItem{}, nil
	}

	contents, err := s.contentStore.GetContentsByIDs(ctx, contentIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch content details for related search results: %w", err)
	}

	results := make([]SearchResultItem, 0, params.Limit)
	for _, content := range contents {
		if content == nil {
			log.Printf("WARN: Related content not found for ID returned by vector search")
			continue
		}
		score, ok := scoresMap[content.ID]
		if !ok {
			log.Printf("WARN: Score not found for related content ID %d", content.ID)
			continue
		}
		results = append(results, SearchResultItem{
			Content: content,
			Score:   score,
		})
		if len(results) >= params.Limit {
			break
		}
	}

	return results, nil
}

package apihandlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"mimir/internal/app"
	"mimir/internal/models"
	"mimir/internal/services"
	"mimir/internal/store"

	"github.com/gin-gonic/gin"
)

type APIHandler struct {
	App *app.App
}

func (h *APIHandler) AddContentHandler(c *gin.Context) {
	req, err := parseAddContentRequest(c)
	if err != nil {
		BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	params := services.AddContentParams{
		SourceName: req.Source,
		Title:      req.Title,
		RawInput:   req.Input,
		SourceType: "api",
	}

	content, existed, err := h.App.ContentService.AddContent(c.Request.Context(), params)
	if err != nil {
		Internal(c, fmt.Sprintf("AddContentHandler: failed to add content: %v", err))
		return
	}

	// Display summary and tags in the response if present
	h.respondWithAddContentAndTags(c, content, existed, req.Source)
}

// respondWithAddContentAndTags writes the AddContent response as JSON, including tags and summary if present.
func (h *APIHandler) respondWithAddContentAndTags(c *gin.Context, content *models.Content, existed bool, source string) {
	logMsg := fmt.Sprintf("API AddContent: content_id=%d, existed=%v, title=%q, source=%q", content.ID, existed, content.Title, source)
	fmt.Println(logMsg)

	// Fetch tags for the content
	var tags []*models.Tag
	if h.App.TagService != nil {
		tags, _ = h.App.TagService.GetContentTags(c.Request.Context(), content.ID)
	}

	resp := struct {
		Content models.Content `json:"content"`
		Existed bool           `json:"existed"`
		Tags    []*models.Tag  `json:"tags,omitempty"`
		Summary *string        `json:"summary,omitempty"`
	}{
		Content: *content,
		Existed: existed,
		Tags:    tags,
		Summary: content.Summary,
	}

	status := http.StatusCreated
	if existed {
		status = http.StatusOK
	}
	c.JSON(status, gin.H{"data": resp})
}

// respondWithAddContent writes the AddContent response as JSON.
func (h *APIHandler) respondWithAddContent(c *gin.Context, content *models.Content, existed bool, source string) {
	logMsg := fmt.Sprintf("API AddContent: content_id=%d, existed=%v, title=%q, source=%q", content.ID, existed, content.Title, source)
	fmt.Println(logMsg)

	resp := AddContentResponse{
		Content: *content,
		Existed: existed,
	}

	status := http.StatusCreated
	if existed {
		status = http.StatusOK
	}
	c.JSON(status, gin.H{"data": resp})
}

// parseAddContentRequest parses and validates the AddContentRequest from the JSON body.
func parseAddContentRequest(c *gin.Context) (AddContentRequest, error) {
	var req AddContentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return req, err
	}
	if req.Source == "" || req.Title == "" || req.Input == "" {
		return req, fmt.Errorf("missing required fields: source, title, and input")
	}
	return req, nil
}

func (h *APIHandler) ListContentHandler(c *gin.Context) {
	params, err := h.parseAndValidateListContentParams(c)
	if err != nil {
		BadRequest(c, "Invalid query parameters: "+err.Error())
		return
	}

	items, err := h.App.ContentService.ListContent(c.Request.Context(), params)
	if err != nil {
		Internal(c, fmt.Sprintf("ListContentHandler: failed to list content: %v", err))
		return
	}

	h.respondWithContentItems(c, items)
}

// parseAndValidateListContentParams parses and validates query parameters for listing content.
func (h *APIHandler) parseAndValidateListContentParams(c *gin.Context) (services.ListContentParams, error) {
	limit := 20
	offset := 0
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")
	filterTags := []string{}

	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		} else {
			return services.ListContentParams{}, fmt.Errorf("invalid limit: %s", l)
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		} else {
			return services.ListContentParams{}, fmt.Errorf("invalid offset: %s", o)
		}
	}
	if tagsParam := c.Query("tags"); tagsParam != "" {
		for _, t := range strings.Split(tagsParam, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				filterTags = append(filterTags, t)
			}
		}
	}

	return services.ListContentParams{
		Limit:      limit,
		Offset:     offset,
		SortBy:     sortBy,
		SortOrder:  sortOrder,
		FilterTags: filterTags,
	}, nil
}

// respondWithContentItems writes the content items as a JSON response.
func (h *APIHandler) respondWithContentItems(c *gin.Context, items []services.ContentResultItem) {
	c.JSON(http.StatusOK, gin.H{
		"items": items,
	})
}

// GetContentHandler handles GET requests for a single content item by ID.
func (h *APIHandler) GetContentHandler(c *gin.Context) {
	id, err := parseContentIDFromRequest(c)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}

	content, tags, err := h.fetchContentAndTagsForGet(c, id)
	if err != nil {
		return
	}

	resp := GetContentResponse{
		Content: *content,
		Tags:    tags,
	}
	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// fetchContentAndTagsForGet fetches content and tags for GetContentHandler, handling errors.
func (h *APIHandler) fetchContentAndTagsForGet(c *gin.Context, id int64) (*models.Content, []*models.Tag, error) {
	content, err := h.App.ContentService.GetContent(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			NotFound(c, fmt.Sprintf("Content not found with ID: %d", id))
		} else {
			Internal(c, fmt.Sprintf("fetchContentAndTagsForGet: failed to retrieve content: %v", err))
		}
		return nil, nil, fmt.Errorf("fetchContentAndTagsForGet: %w", err)
	}

	tags, err := h.App.TagService.GetContentTags(c.Request.Context(), id)
	if err != nil {
		fmt.Printf("WARN: Failed to retrieve tags for content %d: %v\n", id, err)
		tags = []*models.Tag{}
	}
	return content, tags, nil
}

func (h *APIHandler) SearchContentHandler(c *gin.Context) {
	params, err := h.parseAndValidateSearchContentParams(c)
	if err != nil {
		BadRequest(c, "Invalid query parameters: "+err.Error())
		return
	}

	results, err := h.App.SearchService.SemanticSearch(c.Request.Context(), params)
	if err != nil {
		Internal(c, fmt.Sprintf("SearchContentHandler: semantic search failed: %v", err))
		return
	}

	h.respondWithSemanticSearchResults(c, results)
}

// parseAndValidateSearchContentParams parses and validates query parameters for semantic search.
func (h *APIHandler) parseAndValidateSearchContentParams(c *gin.Context) (services.SemanticSearchParams, error) {
	query := c.Query("query")
	if query == "" {
		return services.SemanticSearchParams{}, fmt.Errorf("missing required 'query' parameter")
	}

	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		} else {
			return services.SemanticSearchParams{}, fmt.Errorf("invalid limit: %s", l)
		}
	}

	filterTags := []string{}
	if tagsParam := c.Query("tags"); tagsParam != "" {
		for _, t := range strings.Split(tagsParam, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				filterTags = append(filterTags, t)
			}
		}
	}

	return services.SemanticSearchParams{
		Query:      query,
		Limit:      limit,
		FilterTags: filterTags,
	}, nil
}

// respondWithSemanticSearchResults writes the semantic search results as a JSON response.
func (h *APIHandler) respondWithSemanticSearchResults(c *gin.Context, results []services.SearchResultItem) {
	type searchResult struct {
		Content *models.Content `json:"content"`
		Score   float64         `json:"score"`
	}

	resp := make([]searchResult, len(results))
	for i, r := range results {
		resp[i] = searchResult{
			Content: r.Content,
			Score:   r.Score,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"results": resp,
	})
}

// KeywordSearchHandler handles GET requests for keyword-based search.
func (h *APIHandler) KeywordSearchHandler(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		BadRequest(c, "Missing required 'query' parameter")
		return
	}

	limit := 10 // Default limit for keyword search
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	filterTags := []string{}
	if tagsParam := c.Query("tags"); tagsParam != "" {
		for _, t := range strings.Split(tagsParam, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				filterTags = append(filterTags, t)
			}
		}
	}

	results, err := h.App.SearchService.KeywordSearch(c.Request.Context(), services.KeywordSearchParams{
		Query:      query,
		FilterTags: filterTags,
		Limit:      limit, // Note: KeywordSearch currently ignores limit/offset
	})
	if err != nil {
		Internal(c, fmt.Sprintf("KeywordSearchHandler: keyword search failed: %v", err))
		return
	}

	// The results are already in the desired format (services.KeywordResultItem)
	c.JSON(http.StatusOK, gin.H{
		"results": results,
	})
}

func (h *APIHandler) CategorizeContentHandler(c *gin.Context) {
	contentID, err := parseContentIDFromRequest(c)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}

	content, tags, err := h.fetchContentAndTagsForCategorization(c, contentID)
	if err != nil {
		return
	}

	existingTagNames := make([]string, len(tags))
	for i, t := range tags {
		existingTagNames[i] = t.Name
	}

	if h.App.CategorizationService == nil {
		Internal(c, "Categorization service is not configured or enabled")
		return
	}

	cats, err := h.App.CategorizationService.CategorizeContent(c.Request.Context(), content.Title, content.Body, existingTagNames)
	if err != nil {
		Internal(c, "Categorization failed: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content_id": contentID,
		"tags":       cats.Tags,
		"category":   cats.Category,
		"confidence": cats.Confidence,
	})
}

// parseContentIDFromRequest parses the content ID from path or query.
func parseContentIDFromRequest(c *gin.Context) (int64, error) {
	idStr := c.Param("id")
	if idStr == "" {
		idStr = c.Query("id")
	}
	if idStr == "" {
		return 0, fmt.Errorf("Missing content ID parameter (expected in path /:id or query ?id=)")
	}
	contentID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("Invalid content ID format: %s", idStr)
	}
	return contentID, nil
}

// fetchContentAndTagsForCategorization fetches content and tags, and handles errors.
func (h *APIHandler) fetchContentAndTagsForCategorization(c *gin.Context, contentID int64) (*models.Content, []*models.Tag, error) {
	content, err := h.App.ContentService.GetContent(c.Request.Context(), contentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			NotFound(c, fmt.Sprintf("Content not found with ID: %d", contentID))
		} else {
			Internal(c, fmt.Sprintf("fetchContentAndTagsForCategorization: failed to fetch content: %v", err))
		}
		return nil, nil, fmt.Errorf("fetchContentAndTagsForCategorization: %w", err)
	}

	tags, err := h.App.TagService.GetContentTags(c.Request.Context(), contentID)
	if err != nil {
		fmt.Printf("WARN: Failed to retrieve tags for content %d during categorization: %v\n", contentID, err)
		tags = []*models.Tag{}
	}
	return content, tags, nil
}

func (h *APIHandler) BatchCategorizeHandler(c *gin.Context) {
	req, err := parseBatchCategorizeRequest(c)
	if err != nil {
		BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	if h.App.CategorizationService == nil {
		Internal(c, "Categorization service is not configured or enabled")
		return
	}

	resultsMap, err := h.App.CategorizationService.BatchCategorize(c.Request.Context(), req.ContentIDs)
	if err != nil {
		Internal(c, "Batch categorization failed: "+err.Error())
		return
	}

	resp := make(map[string]interface{}, len(resultsMap))
	for id, cats := range resultsMap {
		resp[strconv.FormatInt(id, 10)] = gin.H{
			"tags":       cats.Tags,
			"category":   cats.Category,
			"confidence": cats.Confidence,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"results": resp,
	})
}

// parseBatchCategorizeRequest parses and validates the batch categorize request.
func parseBatchCategorizeRequest(c *gin.Context) (struct {
	ContentIDs []int64 `json:"content_ids"`
}, error) {
	var req struct {
		ContentIDs []int64 `json:"content_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		return req, err
	}
	if len(req.ContentIDs) == 0 {
		return req, fmt.Errorf("no content IDs provided")
	}
	return req, nil
}

// AddContentRequest represents the JSON body to add new content
type AddContentRequest struct {
	Source string `json:"source"` // Name of the source (e.g., "Web Upload", "API Import")
	Title  string `json:"title"`  // Title of the content
	Input  string `json:"input"`  // The raw input: file path, URL, or text content
	// ContentType is removed, it will be detected by the processor
}

// AddContentResponse represents the JSON response after adding content
type AddContentResponse struct {
	Content models.Content `json:"content"`
	Existed bool           `json:"existed"`
}

// GetContentResponse represents the JSON response for a single content item
type GetContentResponse struct {
	Content models.Content `json:"content"`
	Tags    []*models.Tag  `json:"tags"`
}

type DummyContentService struct{}

type DummySearchService struct{}

func NewAPIHandler(app *app.App) *APIHandler {
	return &APIHandler{App: app}
}

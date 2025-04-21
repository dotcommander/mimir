package services

import (
	// "bytes" // No longer needed if using FilePath
	"context"
	"fmt"
	"os"
	"io" // Keep io import
	"mimir/internal/config" // Import config package

	"mimir/internal/store"  // Add store import

	"github.com/sashabaranov/go-openai"
	log "github.com/sirupsen/logrus"
)

// OpenAIBatchProvider implements the BatchAPIProvider interface using the OpenAI client.
type OpenAIBatchProvider struct {
	client    *openai.Client
	costStore store.CostTrackingStore
	pricing   map[string]config.PricingInfo // Use config.PricingInfo directly
}

// NewOpenAIBatchProvider creates a new provider for OpenAI Batch API operations.
func NewOpenAIBatchProvider(apiKey string, costStore store.CostTrackingStore, pricing map[string]config.PricingInfo) (*OpenAIBatchProvider, error) { // Update signature
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY") // Fallback to env var
	}
	if apiKey == "" {
		log.Warn("OpenAI API key not provided. OpenAI Batch API provider will be disabled.")
		return &OpenAIBatchProvider{client: nil}, nil // Return disabled provider
		// Or return error: return nil, fmt.Errorf("OpenAI API key not provided for Batch API")
	}

	client := openai.NewClient(apiKey)
	log.Info("OpenAI Batch API provider initialized.")
	return &OpenAIBatchProvider{
		client:    client,
		costStore: costStore,
		pricing:   pricing,
	}, nil
}

// CreateFile uploads a file to OpenAI for batch processing.
func (p *OpenAIBatchProvider) CreateFile(ctx context.Context, fileName string, fileContent []byte) (string, error) {
	if p.client == nil {
		return "", fmt.Errorf("OpenAI Batch API provider is not initialized (missing API key)")
	}

	// --- Use FilePath (Deprecated) - Requires Temp File ---
	tmpFile, err := os.CreateTemp("", "openai_batch_*.jsonl")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file for batch upload: %w", err)
	}
	defer os.Remove(tmpFile.Name()) // Clean up the temp file

	if _, err := tmpFile.Write(fileContent); err != nil {
		tmpFile.Close() // Close before returning error
		return "", fmt.Errorf("failed to write to temporary file '%s': %w", tmpFile.Name(), err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temporary file '%s': %w", tmpFile.Name(), err)
	}

	req := openai.FileRequest{
		FileName: fileName,
		FilePath: tmpFile.Name(), // Use the temporary file path
		Purpose:  "batch",
		// Reader field is not used when FilePath is provided
	}
	// --- End FilePath Usage ---

	file, err := p.client.CreateFile(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create OpenAI file '%s': %w", fileName, err)
	}
	return file.ID, nil
}

// CreateBatch creates a new batch job on OpenAI.
// Corrected return type to openai.BatchResponse to match interface
func (p *OpenAIBatchProvider) CreateBatch(ctx context.Context, inputFileID, endpoint string, completionWindow string) (openai.BatchResponse, error) {
	if p.client == nil {
		return openai.BatchResponse{}, fmt.Errorf("OpenAI Batch API provider is not initialized (missing API key)")
	}

	// Use the correct request struct: CreateBatchRequest
	req := openai.CreateBatchRequest{
		InputFileID:      inputFileID,
		Endpoint:         openai.BatchEndpoint(endpoint), // Cast string to BatchEndpoint type seems correct
		CompletionWindow: completionWindow,               // Pass the string directly, remove type cast
		// Metadata can be added here if needed
	}

	// Corrected return type to openai.BatchResponse
	batchResponse, err := p.client.CreateBatch(ctx, req)
	if err != nil {
		return openai.BatchResponse{}, fmt.Errorf("failed to create OpenAI batch job for file %s: %w", inputFileID, err)
	}
	return batchResponse, nil
}

// RetrieveBatch retrieves the status and details of an existing batch job.
// Corrected return type to openai.BatchResponse to match interface
func (p *OpenAIBatchProvider) RetrieveBatch(ctx context.Context, batchID string) (openai.BatchResponse, error) {
	if p.client == nil {
		return openai.BatchResponse{}, fmt.Errorf("OpenAI Batch API provider is not initialized (missing API key)")
	}

	// Corrected return type to openai.BatchResponse
	batchResponse, err := p.client.RetrieveBatch(ctx, batchID)
	if err != nil {
		return openai.BatchResponse{}, fmt.Errorf("failed to retrieve OpenAI batch job %s: %w", batchID, err)
	}

	// --- Cost Tracking Instrumentation ---
	// Record cost only when the batch job is completed successfully.
	// Note: OpenAI Batch API currently doesn't directly expose token counts or cost per batch job via API.
	// This is a placeholder assuming future API updates or manual cost calculation based on input file size/model. // Use string comparison for status
	// For now, we log a warning.
	if string(batchResponse.Status) == "completed" && p.costStore != nil {
		log.Warnf("OpenAI Batch API (job %s) completed, but cost/token tracking is not yet supported via API. Manual calculation needed.", batchID)
		// Placeholder for future implementation:
		// if batchResponse.Usage != nil { // Hypothetical usage field
		//     priceInfo, ok := p.pricing[batchResponse.Model] // Hypothetical model field
		//     if ok {
		//         cost := calculateCost(batchResponse.Usage, priceInfo)
		//         logEntry := &models.AIUsageLog{ ... }
		//         if err := p.costStore.RecordUsage(ctx, logEntry); err != nil { ... }
		//     }
		// }
	}
	// --- End Cost Tracking ---

	return batchResponse, nil
}

// GetFileContent retrieves the content of a file generated by a batch job.
func (p *OpenAIBatchProvider) GetFileContent(ctx context.Context, fileID string) ([]byte, error) {
	if p.client == nil {
		return nil, fmt.Errorf("OpenAI Batch API provider is not initialized (missing API key)")
	}

	reader, err := p.client.GetFileContent(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenAI file content for file %s: %w", fileID, err)
	}
	// reader is already an io.ReadCloser, defer Close directly
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenAI file content for file %s: %w", fileID, err)
	}
	return content, nil
}

// Ensure OpenAIBatchProvider implements the interface.
var _ BatchAPIProvider = (*OpenAIBatchProvider)(nil)

package chunking

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"mimir/internal/models"
)


const (
	// DefaultMaxTokens defines a reasonable default if not provided.
	DefaultMaxTokens = 200
	// DefaultOverlap defines a reasonable default if not provided.
	DefaultOverlap = 50
)

// Chunk represents a piece of text with associated metadata.
// Standard Metadata Keys:
//   - parser: (string) The type of chunker used ("fallback", "markdown", "html", "html-fallback", etc.).
//   - chunk_index: (int) The 0-based index of this chunk within the generated sequence for the specific content item.
//   - total_chunks: (int) The total number of chunks generated for the specific content item.
//   - source_heading: (string, optional) The heading text associated with this chunk (e.g., from Markdown).
//   - source_tags: ([]string, optional) The HTML tag hierarchy leading to this chunk.
type Chunk struct {
	Text     string
	// Metadata stores structured information about the chunk's origin and context.
	Metadata map[string]interface{}
}

// ContentAwareChunk selects the appropriate chunking strategy based on content type
// and metadata overrides, then executes it.
func ContentAwareChunk(content *models.Content, maxTokens, overlap int) []Chunk {
	ctx := context.Background() // Use a background context for chunking logic

	// Determine the target chunker type
	targetChunkerType := "fallback" // Default

	// 1. Check for metadata override
	var meta map[string]interface{}
	if content.Metadata != nil && len(content.Metadata) > 0 { // Check length to avoid error on empty json
		if err := json.Unmarshal(content.Metadata, &meta); err == nil {
			if override, ok := meta["chunker"].(string); ok && override != "" {
				log.Printf("Using chunker override '%s' from metadata for content %d", override, content.ID)
				targetChunkerType = strings.ToLower(override)
			}
		} else {
			log.Printf("WARN: Failed to unmarshal content metadata for content %d: %v", content.ID, err)
		}
	}

	// 2. If no override, detect based on ContentType
	if targetChunkerType == "fallback" { // Only detect if not overridden
		contentTypeLower := strings.ToLower(content.ContentType)
		if strings.Contains(contentTypeLower, "markdown") {
			targetChunkerType = "markdown"
		} else if strings.Contains(contentTypeLower, "html") {
			targetChunkerType = "html"
		}
		// Add more detections if needed (e.g., "application/pdf" -> pdf chunker)
	}

	log.Printf("Selected chunker type '%s' for content %d (ContentType: %s)", targetChunkerType, content.ID, content.ContentType)

	// Instantiate the target chunker
	var chunker Chunker
	switch targetChunkerType {
	case "markdown":
		chunker = NewMarkdownChunker()
	case "html":
		chunker = NewHTMLChunker()
	case "fallback":
		fallthrough // Explicit fallthrough
	default:
		if targetChunkerType != "fallback" {
			log.Printf("WARN: Unknown or unsupported chunker type '%s' requested for content %d. Using fallback.", targetChunkerType, content.ID)
		}
		chunker = NewFallbackChunker()
		targetChunkerType = "fallback" // Ensure type reflects actual chunker used
	}

	// Execute the chunking
	chunks, err := chunker.Chunk(ctx, content, maxTokens, overlap)
	if err != nil {
		log.Printf("ERROR: Chunker type '%s' failed for content %d: %v. Attempting fallback.", targetChunkerType, content.ID, err)
		// If the selected chunker failed, explicitly try the fallback chunker
		if targetChunkerType != "fallback" {
			fallbackChunker := NewFallbackChunker()
			chunks, err = fallbackChunker.Chunk(ctx, content, maxTokens, overlap)
			if err != nil {
				// If fallback also fails, log critical error and return empty slice
				log.Printf("CRITICAL: Fallback chunker also failed for content %d: %v", content.ID, err)
				return []Chunk{} // Return empty slice on critical failure
			}
			log.Printf("Successfully used fallback chunker after initial failure for content %d", content.ID)
			targetChunkerType = "fallback" // Update type to reflect fallback was used
		} else {
			// Add metadata even for failed chunks if possible? Maybe not useful.
			// For now, just return empty on critical failure.
			// Ensure metadata includes parser type even on failure?
			// metadata := map[string]interface{}{"parser": targetChunkerType, "error": err.Error()}
			// return []Chunk{{Text: "", Metadata: metadata}} // Or similar error indication
			// Fallback itself failed
			log.Printf("CRITICAL: Fallback chunker failed for content %d: %v", content.ID, err)
			return []Chunk{} // Return empty slice on critical failure
		}
	}

	log.Printf("Successfully chunked content %d using '%s' strategy. Generated %d chunks.", content.ID, targetChunkerType, len(chunks))

	// Ensure parser type and total chunks are set in metadata for all generated chunks
	totalChunks := len(chunks)
	for i := range chunks {
		chunks[i].Metadata["parser"] = targetChunkerType
		chunks[i].Metadata["total_chunks"] = totalChunks
	}
	return chunks
}

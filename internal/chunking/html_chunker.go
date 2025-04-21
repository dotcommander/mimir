package chunking

import (
	"bytes"
	"context" // Add context import
	"log"
	"strings"

	"mimir/internal/models" // Add models import

	"golang.org/x/net/html"
)

// htmlChunker implements the Chunker interface for HTML content.
type htmlChunker struct{}

// NewHTMLChunker creates a new instance of the HTML chunker.
func NewHTMLChunker() *htmlChunker { // Return concrete type *htmlChunker
	return &htmlChunker{}
}

// Chunk splits HTML content into chunks based on block-level elements and token limits.
// It now accepts context and models.Content.
func (c *htmlChunker) Chunk(ctx context.Context, content *models.Content, maxTokens, overlap int) ([]Chunk, error) {
	chunks := []Chunk{}
	body := strings.TrimSpace(content.Body)
	if body == "" {
		return chunks, nil // Return empty slice and nil error for empty content
	}

	// Validate and apply defaults for chunking parameters
	if maxTokens <= 0 { maxTokens = DefaultMaxTokens }
	if overlap < 0 { overlap = DefaultOverlap }
	if overlap >= maxTokens { overlap = maxTokens - 1 }

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		log.Printf("WARN: [ContentID: %d] Failed to parse HTML content, falling back to simple chunking: %v", content.ID, err)
		// Fallback to simple text chunking if HTML parsing fails, indicating parser type
		// Pass content.Body directly to simpleChunkText
		return simpleChunkText(content.Body, maxTokens, overlap, "html-fallback"), nil
	}

	var currentChunk strings.Builder
	var currentTokens int
	chunkIndex := 0
	isBlock := false // Declare isBlock here to be accessible within the function scope
	var currentBlockTags []string // Track tag hierarchy for context

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n == nil {
			return
		}

		// Define tags to generally ignore content within
		ignoreTags := map[string]bool{
			"script": true, "style": true, "head": true, "nav": true,
			"footer": true, "aside": true, "form": true, "noscript": true,
		}
		if n.Type == html.ElementNode && ignoreTags[n.Data] {

			return // Skip this node and all its descendants
		}


		// Add newline before starting a new block element's content, if chunk isn't empty
		if isBlock && currentChunk.Len() > 0 && !strings.HasSuffix(currentChunk.String(), "\n\n") {
			currentChunk.WriteString("\n\n") // Double newline for block separation
		}

		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				// Simple token estimation (split by space)
				words := strings.Fields(text)
				wordCount := len(words)

				// Check if adding this text exceeds maxTokens
				if currentTokens+wordCount > maxTokens && currentChunk.Len() > 0 {
					// Finalize the current chunk
					chunks = append(chunks, Chunk{ // Append the chunk here
						Text: strings.TrimSpace(currentChunk.String()),
						Metadata: map[string]interface{}{ // Base metadata
							"chunk_index": chunkIndex,
							"source_tags": append([]string{}, currentBlockTags...), // Copy current tag hierarchy
						},
		}) // <-- Moved closing parenthesis here
					chunkIndex++

					// Start new chunk with overlap
					overlappedText := getOverlap(currentChunk.String(), overlap)
					currentChunk.Reset()
					currentChunk.WriteString(overlappedText)
					// Add a space if overlap exists and doesn't end with space/newline
					if overlappedText != "" && !strings.HasSuffix(overlappedText, " ") && !strings.HasSuffix(overlappedText, "\n") {
						currentChunk.WriteString(" ")
					}
					currentTokens = len(strings.Fields(overlappedText)) // Recalculate tokens for overlap
				}

				// Add the new text
				if currentChunk.Len() > 0 && !strings.HasSuffix(currentChunk.String(), " ") && !strings.HasSuffix(currentChunk.String(), "\n") {
					currentChunk.WriteString(" ") // Add space separator if needed
				}
				currentChunk.WriteString(text)
				currentTokens += wordCount
			}
		}

		// Track block tag context
		pushedTag := false
		isBlock = isBlockElement(n) // Check if current node is block
		if isBlock {
			currentBlockTags = append(currentBlockTags, n.Data)

			pushedTag = true
		}

		// Recursively traverse children
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			traverse(child)
		}

		// Pop tag context and add newline after ending a block element, if needed for separation
		if isBlock && currentChunk.Len() > 0 && !strings.HasSuffix(currentChunk.String(), "\n\n") {
			// Check if the next sibling is also a block or if it's the end
			isNextBlock := false
			if n.NextSibling != nil {
				isNextBlock = isBlockElement(n.NextSibling)
			}
			// Only add separator if the next element isn't also a block or if it's the last child of its parent
			if !isNextBlock || n.NextSibling == nil {
				currentChunk.WriteString("\n\n")
			}
		}
		// Pop the tag context after processing children and adding separator
		if pushedTag && len(currentBlockTags) > 0 {
			currentBlockTags = currentBlockTags[:len(currentBlockTags)-1]
		}
	}

	// Add the last remaining chunk if it's not empty
	if currentChunk.Len() > 0 {
		chunks = append(chunks, Chunk{
			Text: strings.TrimSpace(currentChunk.String()),
			Metadata: map[string]interface{}{ // Base metadata
				"chunk_index": chunkIndex,
				"source_tags": append([]string{}, currentBlockTags...), // Final tag hierarchy
			},
		})
	}

	// If parsing resulted in no chunks but content exists, fallback
	if len(chunks) == 0 && body != "" {
		log.Printf("WARN: [ContentID: %d] HTML parsing yielded no chunks for non-empty content. Falling back.", content.ID)
		return simpleChunkText(content.Body, maxTokens, overlap, "html-fallback"), nil // Indicate fallback parser type
	}

	// Start traversal from the document root
	traverse(doc)
	return chunks, nil // Return chunks and nil error on success
}

// isBlockElement checks if an HTML node represents a common block-level element.
// This list can be expanded based on desired chunking behavior.
func isBlockElement(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	// Consider adding more block elements like form, fieldset, dl, dt, dd etc. if needed
	switch n.Data {
	case "address", "article", "aside", "blockquote", "canvas", "dd", "div", "dl", "dt", "fieldset", "figcaption", "figure", "footer", "form", "h1", "h2", "h3", "h4", "h5", "h6", "header", "hr", "li", "main", "nav", "noscript", "ol", "p", "pre", "section", "table", "tfoot", "ul", "video":
		return true
	default:
		return false
	}
}

// Helper function to extract text content from an HTML node tree.
// Note: This is a simplified text extraction. The main logic iterates directly.
func extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		// Replace non-breaking spaces and trim
		return strings.TrimSpace(strings.ReplaceAll(n.Data, "\u00A0", " "))
	}
	if n.Type != html.ElementNode {
		return ""
	}
	// Skip script and style content
	if n.Data == "script" || n.Data == "style" {
		return ""
	}

	var buf bytes.Buffer
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		buf.WriteString(extractText(c))
		// Add space or newline based on block elements if needed for pure text extraction
		if c.NextSibling != nil {
			if isBlockElement(c) || isBlockElement(c.NextSibling) {
				buf.WriteString("\n") // Add newline if either current or next is block
			} else {
				buf.WriteString(" ") // Default space separator for inline elements
			}
		}
	}
	// Trim spaces and handle potential multiple newlines
	// Replace multiple spaces/newlines with single ones for cleaner output
	cleaned := strings.Join(strings.Fields(buf.String()), " ")
	return cleaned
}

// --- Fallback Simple Chunker (copied from markdown_chunker.go for self-containment if needed) ---
// simpleChunkText splits plain text into chunks based on token count.
// It now returns []Chunk directly (no error) as it's a fallback.
func simpleChunkText(text string, maxTokens, overlap int, parserType string) []Chunk {
	var chunks []Chunk
	words := strings.Fields(text) // Simple tokenization by space
	totalWords := len(words)
	startIndex := 0
	chunkIndex := 0

	if totalWords == 0 {
		return chunks // Return empty if no words
	}

	for startIndex < totalWords {
		endIndex := startIndex + maxTokens
		if endIndex > totalWords {
			endIndex = totalWords
		}

		// Ensure endIndex doesn't somehow become less than startIndex
		if endIndex <= startIndex {
			if startIndex < totalWords { // If there's still text left, take at least one word
				endIndex = startIndex + 1
			} else {
				break // Should not happen, but safety break
			}
		}


		chunkText := strings.Join(words[startIndex:endIndex], " ")
		chunks = append(chunks, Chunk{
			Text: chunkText,
			Metadata: map[string]interface{}{
				"parser":      parserType, // Indicate fallback or simple type
				"chunk_index": chunkIndex,
			},
		})
		chunkIndex++

		// Calculate next start index based on overlap
		nextStart := startIndex + maxTokens - overlap

		// If overlap is too large or maxTokens too small, ensure progress
		if nextStart <= startIndex {
			nextStart = startIndex + 1 // Ensure we move forward by at least one word
		}

		startIndex = nextStart

		// Break if startIndex goes beyond totalWords (already handled by loop condition, but extra safety)
		if startIndex >= totalWords {
			break
		}
	}
	return chunks
}


// getOverlap extracts the last 'overlap' words from a text chunk.
func getOverlap(text string, overlap int) string {
	trimmedText := strings.TrimSpace(text)
	if overlap <= 0 || trimmedText == "" {
		return ""
	}
	words := strings.Fields(trimmedText)
	if len(words) <= overlap {
		return trimmedText // Return the whole text if it's shorter than the overlap
	}
	startIndex := len(words) - overlap
	return strings.Join(words[startIndex:], " ")
}

// Ensure htmlChunker implements the Chunker interface.
var _ Chunker = (*htmlChunker)(nil)

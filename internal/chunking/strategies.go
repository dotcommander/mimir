package chunking

import (
	"context" // Add context import
	"log"
	"strings"
	"regexp"  // Add regexp import
	// "unicode" // Remove unused unicode import
	"github.com/neurosnap/sentences" // Add this import
	// "golang.org/x/net/html" // Remove unused html import
	"mimir/internal/models" // Add models import
)

// Chunker defines the interface for different chunking strategies.
type Chunker interface {
	Chunk(ctx context.Context, content *models.Content, maxTokens, overlap int) ([]Chunk, error) // Keep signature
}

// calculateSentenceOverlap finds sentences at the end of a text block
// that approximate the desired token overlap count.
func calculateSentenceOverlap(text string, overlapTokens int) string {
	if overlapTokens <= 0 || text == "" {
		return ""
	}

	tokenizer := sentences.NewSentenceTokenizer(nil) // Use default locale
	if tokenizer == nil { // Check if tokenizer creation failed (though unlikely with nil locale)
		log.Printf("WARN: Failed to create sentence tokenizer, falling back to word overlap.")
		words := strings.Fields(text) // Fallback to simple word overlap
		if overlapTokens > len(words) { // Adjust overlap if it exceeds word count
			overlapTokens = len(words)
		}
		if len(words) == 0 || overlapTokens <= 0 {
			return ""
		}
		return strings.Join(words[len(words)-overlapTokens:], " ") + " "
	}

	sents := tokenizer.Tokenize(text)
	if len(sents) == 0 {
		return "" // No sentences found
	}

	var overlapSentences []string
	accumulatedTokens := 0
	// Iterate backwards through sentences
	for i := len(sents) - 1; i >= 0; i-- {
		sentenceText := strings.TrimSpace(sents[i].Text)
		if sentenceText == "" {
			continue
		}
		sentenceTokens := estimateTokens(sentenceText) // Use existing word count estimator

		// If adding this sentence doesn't exceed overlap, prepend it
		if accumulatedTokens+sentenceTokens <= overlapTokens {
			overlapSentences = append([]string{sentenceText}, overlapSentences...) // Prepend
			accumulatedTokens += sentenceTokens
		} else {
			// If adding this sentence *would* exceed, but we haven't added anything yet,
			// add this single sentence (it's the closest we can get from the end).
			if len(overlapSentences) == 0 {
				overlapSentences = append([]string{sentenceText}, overlapSentences...)
			}
			break // Stop accumulating sentences
		}
	}

	if len(overlapSentences) == 0 {
		return "" // No suitable overlap found
	}

	// Join the selected sentences and add a trailing space
	return strings.Join(overlapSentences, " ") + " "
}

// --- Fallback Chunker (Simple Text Split) ---

type FallbackChunker struct{}

func NewFallbackChunker() *FallbackChunker {
	return &FallbackChunker{}
}

// estimateTokens provides a basic word count estimation.
func estimateTokens(text string) int {
	return len(strings.Fields(text))
}

// Chunk implements a fallback text splitting logic.
// It prioritizes splitting by paragraphs (\n\n), then lines (\n), then words.
func (c *FallbackChunker) Chunk(ctx context.Context, content *models.Content, maxTokens, overlap int) ([]Chunk, error) { // Keep signature
	log.Printf("Using FallbackChunker for content %d (Title: %s)", content.ID, content.Title)
	var finalChunks []Chunk
	text := strings.TrimSpace(content.Body)

	if text == "" {
		log.Printf("FallbackChunker: Content body is empty for content %d.", content.ID)
		return finalChunks, nil
	}

	// Validate and apply defaults for chunking parameters
	if maxTokens <= 0 {
		log.Printf("FallbackChunker: Invalid maxTokens (%d), using default %d for content %d", maxTokens, DefaultMaxTokens, content.ID)
		maxTokens = DefaultMaxTokens
	}
	if overlap < 0 {
		log.Printf("FallbackChunker: Negative overlap (%d) is invalid, using default %d for content %d", overlap, DefaultOverlap, content.ID)
		overlap = DefaultOverlap
	}
	if overlap >= maxTokens {
		log.Printf("FallbackChunker: Overlap (%d) >= maxTokens (%d), adjusting overlap to %d for content %d", overlap, maxTokens, maxTokens-1, content.ID)
		overlap = maxTokens - 1 // Ensure overlap is less than maxTokens
	}

	// 1. Split by paragraphs (double newline)
	paragraphs := strings.Split(text, "\n\n")
	var intermediateChunks []string // Store text pieces before applying overlap

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		paraTokens := estimateTokens(para)
		if paraTokens <= maxTokens {
			// Paragraph fits, add it as is
			intermediateChunks = append(intermediateChunks, para)
		} else {
			// 2. Paragraph too long, split by lines (single newline)
			lines := strings.Split(para, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				lineTokens := estimateTokens(line)
				if lineTokens <= maxTokens {
					// Line fits, add it
					intermediateChunks = append(intermediateChunks, line)
				} else {
					// 3. Line too long, split by words (ultimate fallback)
					words := strings.Fields(line)
					numWords := len(words)
					startIndex := 0
					for startIndex < numWords {
						// Estimate end index based on maxTokens (treating tokens as words here)
						endIndex := startIndex + maxTokens
						if endIndex > numWords {
							endIndex = numWords
						}
						wordChunk := strings.Join(words[startIndex:endIndex], " ")
						intermediateChunks = append(intermediateChunks, wordChunk)
						// Move start index for the next word chunk. No overlap needed at this word level.
						startIndex = endIndex
					}
				}
			}
		}
	}

	// 4. Apply Overlap and create final Chunks from intermediateChunks
	if len(intermediateChunks) == 0 {
		log.Printf("FallbackChunker: No intermediate chunks generated for content %d.", content.ID)
		return finalChunks, nil
	}

	currentChunk := ""
	currentTokens := 0
	chunkIndex := 0

	for i, piece := range intermediateChunks {
		pieceTokens := estimateTokens(piece)

		// If adding the next piece exceeds maxTokens, finalize the current chunk
		if currentTokens > 0 && currentTokens+pieceTokens > maxTokens {
			finalizedChunkText := strings.TrimSpace(currentChunk) // Get text before adding metadata
			metadata := map[string]interface{}{"chunk_index": chunkIndex} // Base metadata
			finalChunks = append(finalChunks, Chunk{Text: finalizedChunkText, Metadata: metadata})
			chunkIndex++
			// Start new chunk with sentence-based overlap from the *previous* chunk's text
			overlapText := calculateSentenceOverlap(finalizedChunkText, overlap) // Use helper
			currentChunk = overlapText
			currentTokens = estimateTokens(overlapText)

			// Safeguard check (remains the same)
			// If the current piece *still* doesn't fit after overlap reset,
			// it means the piece itself is larger than maxTokens minus overlap.
			// This shouldn't happen with the word splitting above, but as a safeguard:
			if currentTokens+pieceTokens > maxTokens && currentTokens > 0 {
				// Force finalize the overlap chunk and start fresh with the large piece // Base metadata + warning
				metadata := map[string]interface{}{"chunk_index": chunkIndex, "warning": "Overlap truncated due to large piece following"}
				finalChunks = append(finalChunks, Chunk{Text: strings.TrimSpace(currentChunk), Metadata: metadata})
				chunkIndex++
				currentChunk = ""
				currentTokens = 0
			}
		}

		// Add the piece to the current chunk
		if currentChunk != "" && !strings.HasSuffix(currentChunk, " ") {
			currentChunk += " " // Add space if needed
		}
		currentChunk += piece
		currentTokens += pieceTokens

		// Handle the very last piece - ensure it gets added as a chunk
		if i == len(intermediateChunks)-1 && strings.TrimSpace(currentChunk) != "" {
			metadata := map[string]interface{}{"chunk_index": chunkIndex} // Base metadata
			finalChunks = append(finalChunks, Chunk{Text: strings.TrimSpace(currentChunk), Metadata: metadata})
			chunkIndex++
		}
	}

	log.Printf("FallbackChunker generated %d chunks for content %d", len(finalChunks), content.ID)
	return finalChunks, nil
}

// --- Markdown Chunker (Stub) ---

type MarkdownChunker struct{}

func NewMarkdownChunker() *MarkdownChunker {
	return &MarkdownChunker{}
}

// Chunk implements Markdown-specific chunking.
// It splits the content by headings (##, ###, etc.) and then chunks each section.
func (c *MarkdownChunker) Chunk(ctx context.Context, content *models.Content, maxTokens, overlap int) ([]Chunk, error) { // Keep signature
	log.Printf("Using MarkdownChunker for content %d (Title: %s)", content.ID, content.Title)
	var finalChunks []Chunk
	text := strings.TrimSpace(content.Body)

	if text == "" {
		log.Printf("MarkdownChunker: Content body is empty for content %d.", content.ID)
		return finalChunks, nil
	}

	// Validate and apply defaults for chunking parameters (same as FallbackChunker)
	if maxTokens <= 0 {
		log.Printf("MarkdownChunker: Invalid maxTokens (%d), using default %d for content %d", maxTokens, DefaultMaxTokens, content.ID)
		maxTokens = DefaultMaxTokens
	}
	if overlap < 0 {
		log.Printf("MarkdownChunker: Negative overlap (%d) is invalid, using default %d for content %d", overlap, DefaultOverlap, content.ID)
		overlap = DefaultOverlap
	}
	if overlap >= maxTokens {
		log.Printf("MarkdownChunker: Overlap (%d) >= maxTokens (%d), adjusting overlap to %d for content %d", overlap, maxTokens, maxTokens-1, content.ID)
		overlap = maxTokens - 1
	}

	// Regex to find markdown headings (##, ###, etc. at the start of a line)
	// (?m) enables multi-line mode
	re := regexp.MustCompile(`(?m)^#{2,}\s+(.*)$`)
	matches := re.FindAllStringSubmatchIndex(text, -1)

	// lastIndex := 0 // Keep commented out or remove
	chunkIndex := 0 // Overall chunk index across all sections

	processSection := func(sectionText string, heading string) {
		log.Printf("MarkdownChunker: Processing section (Heading: '%s', Length: %d) for content %d", heading, len(sectionText), content.ID)
		sectionText = strings.TrimSpace(sectionText)
		if sectionText == "" {
			return
		}

		// Use logic similar to FallbackChunker to chunk the *content* of this section
		paragraphs := strings.Split(sectionText, "\n\n")
		var intermediateChunks []string

		for _, para := range paragraphs {
			// (Identical paragraph/line/word splitting logic as in FallbackChunker)
			para = strings.TrimSpace(para)
			if para == "" { continue }
			paraTokens := estimateTokens(para)
			if paraTokens <= maxTokens {
				intermediateChunks = append(intermediateChunks, para)
			} else {
				lines := strings.Split(para, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" { continue }
					lineTokens := estimateTokens(line)
					if lineTokens <= maxTokens {
						intermediateChunks = append(intermediateChunks, line)
					} else {
						words := strings.Fields(line)
						numWords := len(words)
						startIndex := 0
						for startIndex < numWords {
							endIndex := startIndex + maxTokens
							if endIndex > numWords { endIndex = numWords }
							wordChunk := strings.Join(words[startIndex:endIndex], " ")
							intermediateChunks = append(intermediateChunks, wordChunk)
							startIndex = endIndex
						}
					}
				}
			}
		}

		// Apply overlap logic to the intermediate chunks of this section
		currentChunk := ""
		currentTokens := 0
		for i, piece := range intermediateChunks {
			pieceTokens := estimateTokens(piece)
			if currentTokens > 0 && currentTokens+pieceTokens > maxTokens {
				finalizedChunkText := strings.TrimSpace(currentChunk)
				metadata := map[string]interface{}{"chunk_index": chunkIndex, "source_heading": heading}
				finalChunks = append(finalChunks, Chunk{Text: finalizedChunkText, Metadata: metadata})
				chunkIndex++
				overlapText := calculateSentenceOverlap(finalizedChunkText, overlap)
				currentChunk = overlapText
				currentTokens = estimateTokens(overlapText)
				// Safeguard check (same as fallback)
				if currentTokens+pieceTokens > maxTokens && currentTokens > 0 {
					metadata := map[string]interface{}{"chunk_index": chunkIndex, "source_heading": heading, "warning": "Overlap truncated"}
					finalChunks = append(finalChunks, Chunk{Text: strings.TrimSpace(currentChunk), Metadata: metadata})
					chunkIndex++
					currentChunk = ""
					currentTokens = 0
				}
			}
			if currentChunk != "" && !strings.HasSuffix(currentChunk, " ") { currentChunk += " " }
			currentChunk += piece
			currentTokens += pieceTokens
			if i == len(intermediateChunks)-1 && strings.TrimSpace(currentChunk) != "" {
				metadata := map[string]interface{}{"chunk_index": chunkIndex, "source_heading": heading}
				finalChunks = append(finalChunks, Chunk{Text: strings.TrimSpace(currentChunk), Metadata: metadata})
				chunkIndex++
			}
		}
	}

	// Process content before the first heading (if any)
	if len(matches) == 0 {
		// No headings found, treat the whole content as one section
		processSection(text, "") // No heading associated
	} else {
		// Process the section before the first heading
		processSection(text[0:matches[0][0]], "") // No heading

		// Process sections between headings
		for i := 0; i < len(matches); i++ {
			start := matches[i][1] // End of the heading line itself
			headingText := strings.TrimSpace(text[matches[i][2]:matches[i][3]])

			var end int
			if i+1 < len(matches) {
				end = matches[i+1][0] // Start of the next heading
			} else {
				end = len(text) // End of the entire text
			}
			processSection(text[start:end], headingText)
		}
	}

	log.Printf("MarkdownChunker generated %d chunks for content %d", len(finalChunks), content.ID)
	return finalChunks, nil
}

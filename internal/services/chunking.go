package services

import (
	"regexp"
	"strings"
	"unicode"
)

// ContentAwareChunk splits content into chunks of maxTokens length, with overlap between chunks.
// It is markdown-aware: it tries to split by markdown headings first, then by sentences, then by words.
func ContentAwareChunk(content string, maxTokens, overlap int) []string {
	if maxTokens <= 0 {
		return []string{content}
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return []string{}
	}

	// 1. Try to split by markdown headings (##, ###, etc.)
	sections := splitByMarkdownHeadings(content)
	if len(sections) > 1 {
		return chunkSections(sections, maxTokens)
	}

	// 2. Fallback: split by sentences (using punctuation)
	sentences := splitBySentences(content)
	if len(sentences) > 1 {
		return chunkSentences(sentences, maxTokens, overlap)
	}

	// 3. Fallback: word-based chunking (original logic)
	words := splitWordsUnicode(content)
	if len(words) == 0 {
		return []string{}
	}
	var chunks []string
	start := 0
	for start < len(words) {
		end := start + maxTokens
		if end > len(words) {
			end = len(words)
		}
		chunk := strings.Join(words[start:end], " ")
		chunks = append(chunks, chunk)
		start = end - overlap
		if start < 0 {
			start = 0
		}
	}
	return chunks
}

// splitByMarkdownHeadings splits content by markdown headings (##, ###, etc.)
// Returns a slice of sections (with headings included).
func splitByMarkdownHeadings(content string) []string {
	// Use regex to split on lines starting with 2 or more # (##, ###, etc.)
	re := regexp.MustCompile(`(?m)^#{2,} .*$`)
	indices := re.FindAllStringIndex(content, -1)
	if len(indices) == 0 {
		return []string{content}
	}
	var sections []string
	last := 0
	for i, idx := range indices {
		if idx[0] > last {
			sections = append(sections, strings.TrimSpace(content[last:idx[0]]))
		}
		// If this is the last heading, take the rest of the content
		if i == len(indices)-1 {
			sections = append(sections, strings.TrimSpace(content[idx[0]:]))
		}
		last = idx[0]
	}
	return filterEmpty(sections)
}

// chunkSections chunks each section to maxTokens, returns all chunks.
func chunkSections(sections []string, maxTokens int) []string {
	var chunks []string
	for _, section := range sections {
		words := splitWordsUnicode(section)
		if len(words) == 0 {
			continue
		}
		start := 0
		for start < len(words) {
			end := start + maxTokens
			if end > len(words) {
				end = len(words)
			}
			chunk := strings.Join(words[start:end], " ")
			chunks = append(chunks, chunk)
			start = end
		}
	}
	return filterEmpty(chunks)
}

// splitBySentences splits content into sentences using punctuation.
func splitBySentences(content string) []string {
	// Use a simple regex for sentence boundaries (., !, ? followed by space or newline)
	re := regexp.MustCompile(`(?m)([^.!?]+[.!?])`)
	matches := re.FindAllString(content, -1)
	if len(matches) == 0 {
		return []string{content}
	}
	return filterEmpty(matches)
}

// chunkSentences chunks sentences into groups of maxTokens (tokens = sentences here).
func chunkSentences(sentences []string, maxTokens, overlap int) []string {
	var chunks []string
	start := 0
	for start < len(sentences) {
		end := start + maxTokens
		if end > len(sentences) {
			end = len(sentences)
		}
		chunk := strings.TrimSpace(strings.Join(sentences[start:end], " "))
		chunks = append(chunks, chunk)
		start = end - overlap
		if start < 0 {
			start = 0
		}
	}
	return filterEmpty(chunks)
}

// splitWordsUnicode splits a string into words using unicode.IsSpace.
func splitWordsUnicode(s string) []string {
	var words []string
	var word []rune
	for _, r := range s {
		if unicode.IsSpace(r) {
			if len(word) > 0 {
				words = append(words, string(word))
				word = word[:0]
			}
		} else {
			word = append(word, r)
		}
	}
	if len(word) > 0 {
		words = append(words, string(word))
	}
	return words
}

// filterEmpty removes empty strings from a slice.
func filterEmpty(ss []string) []string {
	var out []string
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

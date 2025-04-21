package summarize

import (
	"context"
	"fmt"
	"strings"
)

type SummarizeTransformer struct {
	maxLength int
}

func NewSummarizeTransformer(maxLength int) *SummarizeTransformer {
	return &SummarizeTransformer{
		maxLength: maxLength,
	}
}

func (t *SummarizeTransformer) Transform(ctx context.Context, text string) (string, error) {
	// Temporary implementation until LLM integration is added
	if t.maxLength <= 0 {
		return text, nil
	}

	// Simple sentence boundary detection
	sentences := strings.SplitAfter(text, ".")
	if len(sentences) == 0 {
		return text, nil
	}

	// Take first 2 sentences as placeholder summary
	var summary strings.Builder
	count := 0
	for _, s := range sentences {
		summary.WriteString(s)
		count++
		if count >= 2 || summary.Len() >= t.maxLength {
			break
		}
	}

	if summary.Len() > t.maxLength {
		return summary.String()[:t.maxLength], nil
	}
	return summary.String(), nil
}

func (t *SummarizeTransformer) Info() string {
	return fmt.Sprintf("Summarize transformer (max length: %d)", t.maxLength)
}

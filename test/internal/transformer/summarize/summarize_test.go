package summarize

import (
	"context"
	"testing"

	"slurp/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestSummarizeTransformer_Transform(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		maxLength int
	}{
		{
			name:      "Short text",
			input:     "Short text.",
			expected:  "Short text.",
			maxLength: 25,
		},
		{
			name:      "Long text",
			input:     "This is a very long text that exceeds the maximum length of 25 characters. It should be truncated.",
			expected:  "This is a very long...",
			maxLength: 25,
		},
		{
			name:      "Text longer than limit (257 chars)",
			input:     "This is a text that is exactly 257 characters long to test the boundary condition. This text should be truncated. This is filler to reach the limit. Filler filler filler!",
			expected:  "This is a text that is exactly 257 characters long to test the boundary condition. This text should be truncated. This is filler to reach the limit. Filler filler filler!",
			maxLength: 257,
		},
		{
			name:      "Text 1 char longer than limit (26 chars)",
			input:     "This text is 26 chars long.",
			expected:  "This text is 26 chars...",
			maxLength: 25,
		},
		{
			name:      "Text at the limit (25 chars)",
			input:     "This is a text that is",
			expected:  "This is a text that is",
			maxLength: 25,
		},
		{
			name:      "Text shorter than limit",
			input:     "This is short",
			expected:  "This is short",
			maxLength: 25,
		},
		{
			name:      "Text with 3 char limit",
			input:     "This is a test",
			expected:  "...",
			maxLength: 3,
		},
		{
			name:      "Text with 2 char limit",
			input:     "This is a test",
			expected:  "T...",
			maxLength: 4,
		},
		{
			name:      "Text with 4 char limit",
			input:     "abcd",
			expected:  "abcd",
			maxLength: 4,
		},
		{
			name:      "Text with 4 char limit and spaces",
			input:     "ab cd",
			expected:  "a...",
			maxLength: 4,
		},
		{
			name:      "Text with spaces and 5 char limit",
			input:     "a b c d",
			expected:  "a...",
			maxLength: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summarizer := NewSummarizeTransformer(tt.maxLength)
			content := &model.Content{Body: tt.input}

			transformed, err := summarizer.Transform(context.Background(), content)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, transformed, "Transformed text should match expected")
		})
	}
}

func TestSummarizeTransformer_Info(t *testing.T) {
	summarizer := NewSummarizeTransformer(256)
	assert.Equal(t, "Summarize transformer (truncates to a maximum length)", summarizer.Info())
}

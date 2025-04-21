package categorizer

import (
	"context"
	"errors"
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ChatCompletionCreator defines the minimal interface for OpenAI chat completions.
type ChatCompletionCreator interface {
	CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

// --- Mock OpenAI Client ---
type mockOpenAIClient struct {
	mockResponse openai.ChatCompletionResponse
	mockError    error
}

func (m *mockOpenAIClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if m.mockError != nil {
		return openai.ChatCompletionResponse{}, m.mockError
	}
	return m.mockResponse, nil
}

// --- End Mock OpenAI Client ---

func TestLLMCategorizer_Categorize_Parsing(t *testing.T) {
	// 1. Define expected LLM response content (valid JSON)
	expectedJSON := `{"tags": ["go", "testing", "mock"], "category": "Software Development", "confidence": 0.85}`
	mockResp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Content: expectedJSON,
				},
			},
		},
	}

	// 2. Create mock client configured with the response
	mockClient := &mockOpenAIClient{
		mockResponse: mockResp,
	}

	// 3. Create the categorizer instance with the mock client
	categorizer := NewLLMCategorizer(mockClient, "gpt-test", "dummy prompt {{TITLE}} {{BODY}}")

	// 4. Prepare a dummy request
	req := CategorizationRequest{
		Title: "Test Title",
		Body:  "Test Body",
	}

	// 5. Call the method under test
	result, err := categorizer.Categorize(context.Background(), req)

	// 6. Assert results
	require.NoError(t, err, "Categorize should not return an error for valid JSON")

	assert.Equal(t, []string{"go", "testing", "mock"}, result.SuggestedTags, "Parsed tags do not match")
	assert.Equal(t, "Software Development", result.SuggestedCategory, "Parsed category does not match")
	assert.Equal(t, 0.85, result.Confidence, "Parsed confidence does not match")
}

// Test case for when the LLM returns non-JSON content
func TestLLMCategorizer_Categorize_InvalidJSON(t *testing.T) {
	// 1. Define invalid LLM response content
	invalidJSON := `This is just plain text, not JSON.`
	mockResp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Content: invalidJSON,
				},
			},
		},
	}

	// 2. Create mock client
	mockClient := &mockOpenAIClient{
		mockResponse: mockResp,
	}

	// 3. Create categorizer
	categorizer := NewLLMCategorizer(mockClient, "gpt-test", "dummy prompt")

	// 4. Prepare request
	req := CategorizationRequest{
		Title: "Test Title",
		Body:  "Test Body",
	}

	// 5. Call method
	_, err := categorizer.Categorize(context.Background(), req)

	// 6. Assert an error occurred and check its content
	require.Error(t, err, "Categorize should return an error for invalid JSON")
	assert.Contains(t, err.Error(), "failed to parse LLM response as JSON", "Error message should indicate JSON parsing failure")
	assert.Contains(t, err.Error(), invalidJSON, "Error message should include the raw invalid content") // Check if raw content is included
}

func TestLLMCategorizer_Categorize_MissingFields(t *testing.T) {
	testCases := []struct {
		name         string
		jsonResponse string
		expectedTags []string
		expectedCat  string
		expectedConf float64 // Note: Current code defaults 0 confidence to 1.0
	}{
		{
			name:         "Missing Tags",
			jsonResponse: `{"category": "Tech", "confidence": 0.7}`,
			expectedTags: nil, // Expect nil or empty slice for missing array
			expectedCat:  "Tech",
			expectedConf: 0.7,
		},
		{
			name:         "Missing Category",
			jsonResponse: `{"tags": ["news"], "confidence": 0.9}`,
			expectedTags: []string{"news"},
			expectedCat:  "", // Expect empty string for missing string
			expectedConf: 0.9,
		},
		{
			name:         "Missing Confidence",
			jsonResponse: `{"tags": ["finance"], "category": "Business"}`,
			expectedTags: []string{"finance"},
			expectedCat:  "Business",
			expectedConf: 1.0, // Expect default value applied by current code
		},
		{
			name:         "Missing All Optional",
			jsonResponse: `{}`, // Empty JSON object
			expectedTags: nil,
			expectedCat:  "",
			expectedConf: 1.0, // Expect default value
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 1. Setup mock response
			mockResp := openai.ChatCompletionResponse{
				Choices: []openai.ChatCompletionChoice{
					{
						Message: openai.ChatCompletionMessage{
							Content: tc.jsonResponse,
						},
					},
				},
			}
			mockClient := &mockOpenAIClient{mockResponse: mockResp}
			categorizer := NewLLMCategorizer(mockClient, "gpt-test", "dummy prompt")
			req := CategorizationRequest{Title: "Test", Body: "Test"}

			// 2. Call method
			result, err := categorizer.Categorize(context.Background(), req)

			// 3. Assert results
			require.NoError(t, err, "Categorize should not return an error for valid JSON with missing fields")
			assert.Equal(t, tc.expectedTags, result.SuggestedTags, "Parsed tags mismatch")
			assert.Equal(t, tc.expectedCat, result.SuggestedCategory, "Parsed category mismatch")
			assert.Equal(t, tc.expectedConf, result.Confidence, "Parsed confidence mismatch")
		})
	}
}

func TestLLMCategorizer_Categorize_APIError(t *testing.T) {
	// 1. Define the error the mock should return
	mockErr := errors.New("simulated API error 429 Too Many Requests")

	// 2. Create mock client configured to return the error
	mockClient := &mockOpenAIClient{
		mockError: mockErr,
	}

	// 3. Create categorizer
	categorizer := NewLLMCategorizer(mockClient, "gpt-test", "dummy prompt")

	// 4. Prepare request
	req := CategorizationRequest{
		Title: "Test Title",
		Body:  "Test Body",
	}

	// 5. Call method
	_, err := categorizer.Categorize(context.Background(), req)

	// 6. Assert an error occurred and check it wraps the mock error
	require.Error(t, err, "Categorize should return an error when the API call fails")
	// Check if the returned error wraps the specific mock error or contains its message
	assert.ErrorIs(t, err, mockErr, "Returned error should wrap the original API error")
	assert.Contains(t, err.Error(), "openai chat completion failed", "Error message should indicate the source")
}

// Test case for when the OpenAI API returns an empty Choices slice
func TestLLMCategorizer_Categorize_EmptyResponse(t *testing.T) {
	// 1. Define an empty response
	mockResp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{}, // Empty slice
	}

	// 2. Create mock client
	mockClient := &mockOpenAIClient{
		mockResponse: mockResp,
	}

	// 3. Create categorizer
	categorizer := NewLLMCategorizer(mockClient, "gpt-test", "dummy prompt")

	// 4. Prepare request
	req := CategorizationRequest{
		Title: "Test Title",
		Body:  "Test Body",
	}

	// 5. Call method
	_, err := categorizer.Categorize(context.Background(), req)

	// 6. Assert an error occurred due to no choices
	require.Error(t, err, "Categorize should return an error when API returns no choices")
	assert.Contains(t, err.Error(), "no choices returned from OpenAI", "Error message should indicate no choices")
}

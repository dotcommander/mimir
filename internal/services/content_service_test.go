package services_test

import (
	// "context" // Removed unused import
	// "fmt" // Removed unused import
	// "net/http" // Removed unused import
	// "net/http/httptest" // Removed unused import
	"os"
	// "path/filepath" // Removed unused import
	"testing"

	// "mimir/internal/services" // Removed unused import
	// mock_store "mimir/internal/tests/mocks/store" // Removed unused import

	// "github.com/stretchr/testify/assert" // Removed unused import
	"github.com/stretchr/testify/require"
)

// Helper function to create a temporary file for testing
func createTempFile(t *testing.T, content string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test_prepare_input_*.txt")
	require.NoError(t, err)
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	err = tmpFile.Close()
	require.NoError(t, err)
	return tmpFile.Name()
}

// Removed TestContentService_PrepareContentInput as the method was moved to inputprocessor

// TODO: Add tests for AddContent, ListContent, DeleteContent using mocks

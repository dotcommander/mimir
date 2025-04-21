package service

import (
	"context"
	"testing"

	"slurp/internal/model"
	"slurp/internal/pkg/storage/local"
	"slurp/internal/repository"
	"slurp/internal/transformer"
	"slurp/internal/transformer/extract"
	"slurp/internal/transformer/summarize"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestService(t *testing.T) (*FileTransformerService, *repository.ContentRepository) {
	db := setupTestDB(t)
	repo := repository.NewContentRepository(db)

	tempDir := t.TempDir()
	store, err := local.NewLocalStorage(tempDir)
	require.NoError(t, err)

	tm := transformer.NewManager()
	tm.RegisterTransformer("summarize", summarize.NewSummarizeTransformer(256))
	tm.RegisterTransformer("extract", extract.NewExtractTransformer(5))

	queue := &mockQueue{} // Implement mock queue interface

	return NewFileTransformerService(tm, repo, store, queue), repo
}

func TestProcessFile_Success(t *testing.T) {
	service, repo := setupTestService(t)

	content := &model.Content{
		MD5:    "testmd5",
		Body:   "This is a test content for processing.",
		Size:   100,
		Source: "testsource",
	}

	err := service.ProcessFile(context.Background(), content, []string{"summarize", "extract"})
	assert.NoError(t, err)

	// Verify content was created
	retrieved, err := repo.GetByMD5("testmd5")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)

	// Verify transformers were applied
	transformers, err := repo.GetTransformersByContentID(retrieved.ID)
	assert.NoError(t, err)
	assert.Len(t, transformers, 2)
}

func TestProcessFile_NoTransformers(t *testing.T) {
	service, repo := setupTestService(t)

	content := &model.Content{
		MD5:    "testmd5",
		Body:   "This is a test content for processing.",
		Size:   100,
		Source: "testsource",
	}

	err := service.ProcessFile(context.Background(), content, []string{})
	assert.NoError(t, err)

	// Verify content was created but no transformers
	retrieved, err := repo.GetByMD5("testmd5")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)

	transformers, err := repo.GetTransformersByContentID(retrieved.ID)
	assert.NoError(t, err)
	assert.Len(t, transformers, 0)
}

// mockQueue implements queue.Queue for testing
type mockQueue struct{}

func (m *mockQueue) Enqueue(ctx context.Context, taskName string, payload []byte) error {
	return nil
}

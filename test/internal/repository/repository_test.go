package repository

import (
	"database/sql"
	"testing"
	"time"

	"slurp/internal/model"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	assert.NoError(t, err)

	// Create tables (simplified schema for testing)
	_, err = db.Exec(`
		CREATE TABLE content (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			md5 TEXT,
			body TEXT,
			size INTEGER,
			source TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE meta_options (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content_id INTEGER,
			meta_key TEXT,
			meta_value TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE transformers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content_id INTEGER,
			transformer TEXT,
			body TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	assert.NoError(t, err)

	return db
}

func TestContentRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewContentRepository(db)

	content := &model.Content{
		MD5:    "testmd5",
		Body:   "testbody",
		Size:   100,
		Source: "testsource",
	}

	id, err := repo.Create(content)
	assert.NoError(t, err)
	assert.NotZero(t, id)
}

func TestContentRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewContentRepository(db)

	content := &model.Content{
		MD5:    "testmd5",
		Body:   "testbody",
		Size:   100,
		Source: "testsource",
	}

	id, err := repo.Create(content)
	assert.NoError(t, err)

	retrievedContent, err := repo.GetByID(id)
	assert.NoError(t, err)
	assert.Equal(t, id, retrievedContent.ID)
	assert.Equal(t, "testmd5", retrievedContent.MD5)
	assert.Equal(t, "testbody", retrievedContent.Body)
	assert.Equal(t, 100, retrievedContent.Size)
	assert.Equal(t, "testsource", retrievedContent.Source)
}

func TestContentRepository_GetByMD5(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewContentRepository(db)

	content := &model.Content{
		MD5:    "testmd5",
		Body:   "testbody",
		Size:   100,
		Source: "testsource",
	}

	_, err := repo.Create(content)
	assert.NoError(t, err)

	retrievedContent, err := repo.GetByMD5("testmd5")
	assert.NoError(t, err)
	assert.Equal(t, "testmd5", retrievedContent.MD5)
	assert.Equal(t, "testbody", retrievedContent.Body)
	assert.Equal(t, 100, retrievedContent.Size)
	assert.Equal(t, "testsource", retrievedContent.Source)
}

func TestContentRepository_UpdateBody(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewContentRepository(db)
	content := &model.Content{
		MD5:    "updatemd5",
		Body:   "originalbody",
		Size:   100,
		Source: "testsource",
	}
	id, err := repo.Create(content)
	assert.NoError(t, err)

	err = repo.UpdateBody(id, "updatedbody")
	assert.NoError(t, err)

	updatedContent, err := repo.GetByID(id)
	assert.NoError(t, err)
	assert.Equal(t, "updatedbody", updatedContent.Body)
}

func TestContentRepository_CreateTransformer(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewContentRepository(db)
	content := &model.Content{
		MD5:    "transformermd5",
		Body:   "originalbody",
		Size:   100,
		Source: "testsource",
	}
	contentID, err := repo.Create(content)
	assert.NoError(t, err)

	transformer := &model.Transformer{
		ContentID:   contentID,
		Transformer: "testtransformer",
		Body:        "transformedbody",
	}
	transformerID, err := repo.CreateTransformer(transformer)
	assert.NoError(t, err)
	assert.NotZero(t, transformerID)
}

func TestContentRepository_GetTransformersByContentID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewContentRepository(db)
	content := &model.Content{
		MD5:    "transformermd5",
		Body:   "originalbody",
		Size:   100,
		Source: "testsource",
	}
	contentID, err := repo.Create(content)
	assert.NoError(t, err)

	transformer := &model.Transformer{
		ContentID:   contentID,
		Transformer: "testtransformer",
		Body:        "transformedbody",
	}
	_, err = repo.CreateTransformer(transformer)
	assert.NoError(t, err)

	transformers, err := repo.GetTransformersByContentID(contentID)
	assert.NoError(t, err)
	assert.Len(t, transformers, 1)
	assert.Equal(t, "testtransformer", transformers[0].Transformer)
	assert.Equal(t, "transformedbody", transformers[0].Body)
}

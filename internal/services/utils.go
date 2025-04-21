package services

import (
	"context"

	"github.com/pgvector/pgvector-go"
	"mimir/internal/store" // Add store import
)

type DummyProvider struct {
	NameStr      string
	Model        string
	DimensionVal int
}

func (p *DummyProvider) Name() string                 { return p.NameStr }
func (p *DummyProvider) ModelName() string            { return p.Model }
func (p *DummyProvider) Status() store.ProviderStatus { return store.ProviderStatusActive } // Use store.ProviderStatus
func (p *DummyProvider) Dimension() int               { return p.DimensionVal }

func (p *DummyProvider) GenerateEmbedding(ctx context.Context, text string) (pgvector.Vector, error) {
	// TODO: call real API
	return pgvector.NewVector([]float32{0, 0, 0}), nil
}

func (p *DummyProvider) GenerateEmbeddings(ctx context.Context, texts []string) ([]pgvector.Vector, error) {
	vecs := make([]pgvector.Vector, len(texts))
	for i := range texts {
		vecs[i] = pgvector.NewVector([]float32{0, 0, 0})
	}
	return vecs, nil
}

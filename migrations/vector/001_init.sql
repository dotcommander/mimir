-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE embeddings (
  id UUID PRIMARY KEY,
  content_id BIGINT NOT NULL, -- Changed from INTEGER to BIGINT
  embedding vector(1536) NOT NULL,
  metadata JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_embeddings_content_id ON embeddings(content_id);

-- Add HNSW index for efficient similarity search using L2 distance
CREATE INDEX ON embeddings USING hnsw (embedding vector_l2_ops);

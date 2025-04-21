-- Initial schema migration for Mimir

-- Enable the pgvector extension if it's not already enabled
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS sources (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    url TEXT,
    source_type TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS content (
    id BIGSERIAL PRIMARY KEY,
    source_id BIGINT REFERENCES sources(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    summary TEXT, -- Add summary column
    content_hash TEXT NOT NULL UNIQUE,
    file_path TEXT, -- Add file_path column
    file_size BIGINT, -- Add file_size column
    content_type TEXT,
    metadata JSONB, -- Add metadata column
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tags (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    slug TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS content_tags (
    content_id BIGINT NOT NULL REFERENCES content(id) ON DELETE CASCADE,
    tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (content_id, tag_id)
);

CREATE TABLE IF NOT EXISTS collections (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    is_pinned BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS collection_content (
    collection_id BIGINT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    content_id BIGINT NOT NULL REFERENCES content(id) ON DELETE CASCADE,
    PRIMARY KEY (collection_id, content_id)
);

CREATE TABLE IF NOT EXISTS search_queries (
    id BIGSERIAL PRIMARY KEY,
    query TEXT NOT NULL,
    executed_at TIMESTAMP NOT NULL DEFAULT NOW(),
    results_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS search_results (
    id BIGSERIAL PRIMARY KEY,
    search_query_id BIGINT NOT NULL REFERENCES search_queries(id) ON DELETE CASCADE,
    content_id BIGINT NOT NULL REFERENCES content(id) ON DELETE CASCADE,
    rank INTEGER NOT NULL, -- Rank within the specific search query
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    relevance_score DOUBLE PRECISION NOT NULL
);

CREATE TABLE IF NOT EXISTS embeddings (
    id UUID PRIMARY KEY,
    content_id BIGINT NOT NULL REFERENCES content(id) ON DELETE CASCADE,
    chunk_text TEXT NOT NULL, -- Add column to store the original chunk text
    vector VECTOR(1536), -- pgvector extension required, dimension matches default openai model
    metadata JSONB, -- Add metadata column for chunk info, etc.
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Table to log background job enqueue events
CREATE TABLE IF NOT EXISTS background_jobs (
    id BIGSERIAL PRIMARY KEY,
    job_id UUID NOT NULL UNIQUE, -- Asynq Task ID
    task_type TEXT NOT NULL,
    payload JSONB,
    queue TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'enqueued', -- e.g., enqueued, processing, completed, failed
    related_entity_type TEXT, -- e.g., 'content'
    related_entity_id BIGINT, -- e.g., content.id
    batch_api_job_id TEXT,    -- OpenAI Batch API Job ID
    batch_input_file_id TEXT, -- OpenAI File ID for input
    batch_output_file_id TEXT,-- OpenAI File ID for output (when completed)
    job_data JSONB,           -- Store additional job data like generated chunks
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Table to log AI API usage and costs
CREATE TABLE IF NOT EXISTS ai_usage_logs (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    provider_name TEXT NOT NULL, -- e.g., 'openai', 'gemini'
    service_type TEXT NOT NULL, -- e.g., 'embedding', 'categorization', 'completion', 'summarization'
    model_name TEXT NOT NULL,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cost DOUBLE PRECISION DEFAULT 0.0,
    related_content_id BIGINT REFERENCES content(id) ON DELETE SET NULL, -- Optional link to content
    related_job_id BIGINT REFERENCES background_jobs(id) ON DELETE SET NULL -- Optional link to job
);

-- Optional: Index for querying jobs related to specific entities
CREATE INDEX IF NOT EXISTS idx_background_jobs_related_entity ON background_jobs (related_entity_type, related_entity_id);
-- Optional: Index for querying jobs by status
CREATE INDEX IF NOT EXISTS idx_background_jobs_status ON background_jobs (status);
-- Optional: Index for querying jobs by batch API job ID
CREATE INDEX IF NOT EXISTS idx_background_jobs_batch_api_job_id ON background_jobs (batch_api_job_id);
-- Optional: Index for querying embeddings by content ID
CREATE INDEX IF NOT EXISTS idx_embeddings_content_id ON embeddings (content_id);
-- Optional: Index for AI usage logs
CREATE INDEX IF NOT EXISTS idx_ai_usage_logs_timestamp ON ai_usage_logs (timestamp);

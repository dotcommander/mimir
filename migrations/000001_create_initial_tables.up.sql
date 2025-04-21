
-- Sources Table
CREATE TABLE sources (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT NULL,
    url TEXT NULL,
    source_type VARCHAR(100) NOT NULL, -- e.g., 'manual', 'file', 'web'
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Contents Table
CREATE TABLE contents (
    id BIGSERIAL PRIMARY KEY,
    source_id BIGINT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    content_hash VARCHAR(64) NOT NULL UNIQUE, -- SHA256 hash length
    file_path TEXT NULL,
    file_size BIGINT NULL,
    content_type VARCHAR(100) NOT NULL, -- e.g., 'text/plain', 'application/pdf'
    metadata JSONB NULL,
    embedding_id UUID NULL, -- Link to the representative embedding in the vector DB
    is_embedded BOOLEAN NOT NULL DEFAULT false,
    last_accessed_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (source_id) REFERENCES sources(id) ON DELETE CASCADE -- Or SET NULL/RESTRICT depending on desired behavior
);

-- Index for efficient lookup by source
CREATE INDEX idx_contents_source_id ON contents(source_id);
-- Index for is_embedded status (useful for worker queries)
CREATE INDEX idx_contents_is_embedded ON contents(is_embedded);

-- Tags Table
CREATE TABLE tags (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    slug VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Content-Tags Join Table
CREATE TABLE content_tags (
    content_id BIGINT NOT NULL,
    tag_id BIGINT NOT NULL,
    PRIMARY KEY (content_id, tag_id),
    FOREIGN KEY (content_id) REFERENCES contents(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

-- Collections Table
CREATE TABLE collections (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT NULL,
    is_pinned BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Content-Collections Join Table
CREATE TABLE content_collections (
    content_id BIGINT NOT NULL,
    collection_id BIGINT NOT NULL,
    PRIMARY KEY (content_id, collection_id),
    FOREIGN KEY (content_id) REFERENCES contents(id) ON DELETE CASCADE,
    FOREIGN KEY (collection_id) REFERENCES collections(id) ON DELETE CASCADE
);

-- Search Queries Table
CREATE TABLE search_queries (
    id BIGSERIAL PRIMARY KEY,
    query TEXT NOT NULL,
    results_count INTEGER NOT NULL,
    executed_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Search Results Table
CREATE TABLE search_results (
    id BIGSERIAL PRIMARY KEY,
    search_query_id BIGINT NOT NULL,
    content_id BIGINT NOT NULL,
    relevance_score DOUBLE PRECISION NOT NULL,
    rank INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (search_query_id) REFERENCES search_queries(id) ON DELETE CASCADE,
    FOREIGN KEY (content_id) REFERENCES contents(id) ON DELETE CASCADE
);

-- Index for efficient lookup of results by query
CREATE INDEX idx_search_results_query_id ON search_results(search_query_id);

-- Add a GIN index on a tsvector expression combining title and body
-- using the 'english' text search configuration.
CREATE INDEX idx_contents_fts ON contents USING GIN (to_tsvector('english', title || ' ' || body));

-- Optional: Consider adding a generated tsvector column for potentially better performance
-- if your PostgreSQL version supports it well and updates are frequent.
-- ALTER TABLE contents ADD COLUMN fts_vector tsvector
--     GENERATED ALWAYS AS (to_tsvector('english', title || ' ' || body)) STORED;
-- CREATE INDEX idx_contents_fts_generated ON contents USING GIN (fts_vector);
-- If using a generated column, update the query in KeywordSearchContent to use fts_vector.

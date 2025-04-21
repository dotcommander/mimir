-- Drop the GIN index
DROP INDEX IF EXISTS idx_contents_fts;

-- If you added the generated column, drop it as well:
-- ALTER TABLE contents DROP COLUMN IF EXISTS fts_vector;
-- DROP INDEX IF EXISTS idx_contents_fts_generated;

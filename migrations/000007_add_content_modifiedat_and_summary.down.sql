-- Remove 'modified_at' and 'summary' columns from the content table

ALTER TABLE content
  DROP COLUMN IF EXISTS modified_at,
  DROP COLUMN IF EXISTS summary;

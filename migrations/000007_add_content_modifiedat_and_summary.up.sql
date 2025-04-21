-- Add 'modified_at' and 'summary' columns to the content table

ALTER TABLE content
  ADD COLUMN IF NOT EXISTS modified_at TIMESTAMP NULL,
  ADD COLUMN IF NOT EXISTS summary TEXT NULL;

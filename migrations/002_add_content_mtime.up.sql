-- Add the modified_at column to the content table
ALTER TABLE content ADD COLUMN modified_at TIMESTAMP NULL;

-- Add a comment to the new column for documentation
COMMENT ON COLUMN content.modified_at IS 'File modification time, if applicable';

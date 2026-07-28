-- Track file modification time for skip-on-mtime optimization.
-- When re-scanning, if source_path + file_size + modified_at match,
-- we can skip re-hashing and subprocess calls entirely.
ALTER TABLE documents ADD COLUMN modified_at TEXT;

-- Fast lookup by source path + size + mtime combo
CREATE INDEX idx_documents_source_path_mtime ON documents(source_path, file_size, modified_at);

-- images: logical images by their S3 key (or manifest id)
CREATE TABLE IF NOT EXISTS images (
    id INTEGER PRIMARY KEY,
    digest TEXT NOT NULL,                  -- digest of the original tarball
    base_lv_id INTEGER,           -- thin volume id for base
    size_bytes INTEGER,
    local_path TEXT,         
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- blobs: content-addressed (by sha256) files pulled from S3
CREATE TABLE IF NOT EXISTS blobs (
    digest TEXT PRIMARY KEY,
    local_path TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    etag TEXT NOT NULL UNIQUE,                      -- for validation
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- activations: snapshots (read-write copies) derived from base images
CREATE TABLE IF NOT EXISTS activations (
    id INTEGER PRIMARY KEY,
    image_id INTEGER NOT NULL REFERENCES images(id),
    snap_lv_id INTEGER NOT NULL UNIQUE,
    mount_path TEXT,
    activated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- simple kv lock to avoid multi-process races without external deps
CREATE TABLE IF NOT EXISTS locks (k TEXT PRIMARY KEY, v TEXT);
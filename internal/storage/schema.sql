-- images: logical images by their S3 key (or manifest id)
CREATE TABLE IF NOT EXISTS images (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    digest TEXT NOT NULL UNIQUE,            -- digest of the original tarball
    base_lv_id INTEGER,                           -- thin volume id for base
    size_bytes INTEGER,
    local_path TEXT,
    complete BOOLEAN NOT NULL DEFAULT 0,       -- if all blobs are fetched
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- blobs: content-addressed (by sha256) files pulled from S3
CREATE TABLE IF NOT EXISTS blobs (
    digest TEXT PRIMARY KEY,
    local_path TEXT,
    size_bytes INTEGER,
    etag TEXT NOT NULL UNIQUE,              -- for validation
    complete BOOLEAN NOT NULL DEFAULT 0,        -- fetched or pending
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- image_blobs: relationship table between images and blobs
CREATE TABLE IF NOT EXISTS image_blobs (
    image_id INTEGER NOT NULL REFERENCES images(id),
    blob_digest TEXT NOT NULL REFERENCES blobs(digest),
    PRIMARY KEY (image_id, blob_digest)
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
CREATE TABLE IF NOT EXISTS locks (
    k TEXT PRIMARY KEY,
    v TEXT
);
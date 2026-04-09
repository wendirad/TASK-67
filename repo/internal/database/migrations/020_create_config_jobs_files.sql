CREATE TABLE config_entries (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    key VARCHAR(200) NOT NULL UNIQUE,
    value TEXT NOT NULL,
    description VARCHAR(500),
    canary_percentage INTEGER CHECK (canary_percentage >= 0 AND canary_percentage <= 100),
    updated_by UUID REFERENCES users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_config_key ON config_entries (key);

CREATE TABLE jobs (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    type VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    payload TEXT,
    result TEXT,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    scheduled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_jobs_status_scheduled ON jobs (status, scheduled_at);
CREATE INDEX idx_jobs_type ON jobs (type);

CREATE TABLE file_records (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    filename VARCHAR(500) NOT NULL,
    file_type VARCHAR(20) NOT NULL CHECK (file_type IN ('import', 'export', 'proof', 'backup')),
    sha256_hash VARCHAR(64) NOT NULL,
    size_bytes BIGINT NOT NULL,
    uploaded_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_files_hash ON file_records (sha256_hash);
CREATE INDEX idx_files_type ON file_records (file_type);

CREATE TABLE backups (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    filename VARCHAR(500) NOT NULL,
    size_bytes BIGINT NOT NULL,
    encrypted BOOLEAN NOT NULL DEFAULT true,
    type VARCHAR(20) NOT NULL CHECK (type IN ('full', 'incremental')),
    status VARCHAR(20) NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'completed', 'failed')),
    wal_start_lsn VARCHAR(20),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_backups_status ON backups (status);
CREATE INDEX idx_backups_created_at ON backups (created_at);

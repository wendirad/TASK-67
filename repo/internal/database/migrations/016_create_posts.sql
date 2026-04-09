CREATE TABLE posts (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    title VARCHAR(300) NOT NULL,
    content TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending_review' CHECK (status IN ('pending_review', 'approved', 'rejected', 'flagged', 'removed')),
    reported_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_posts_user ON posts (user_id);
CREATE INDEX idx_posts_status ON posts (status);
CREATE INDEX idx_posts_created_at ON posts (created_at);

CREATE TABLE post_reports (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    post_id UUID NOT NULL REFERENCES posts(id),
    reported_by UUID NOT NULL REFERENCES users(id),
    reason VARCHAR(500) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reports_post ON post_reports (post_id);
CREATE UNIQUE INDEX idx_reports_post_user ON post_reports (post_id, reported_by);

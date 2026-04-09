CREATE TABLE moderation_decisions (
    id UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    post_id UUID NOT NULL REFERENCES posts(id),
    moderator_id UUID NOT NULL REFERENCES users(id),
    action VARCHAR(20) NOT NULL CHECK (action IN ('approve', 'reject', 'remove', 'ban_user', 'warn_user')),
    reason VARCHAR(1000) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mod_decisions_post ON moderation_decisions (post_id);
CREATE INDEX idx_mod_decisions_moderator ON moderation_decisions (moderator_id);

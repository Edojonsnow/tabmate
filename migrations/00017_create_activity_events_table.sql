-- +goose Up
CREATE TABLE activity_events (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type  TEXT        NOT NULL,
    actor_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_name  TEXT        NOT NULL,
    entity_type TEXT        NOT NULL CHECK (entity_type IN ('table', 'split')),
    entity_code TEXT        NOT NULL,
    entity_name TEXT        NOT NULL DEFAULT '',
    metadata    JSONB       NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_activity_events_entity_code ON activity_events(entity_code);
CREATE INDEX idx_activity_events_actor_id    ON activity_events(actor_id);
CREATE INDEX idx_activity_events_created_at  ON activity_events(created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_activity_events_created_at;
DROP INDEX IF EXISTS idx_activity_events_actor_id;
DROP INDEX IF EXISTS idx_activity_events_entity_code;
DROP TABLE IF EXISTS activity_events;

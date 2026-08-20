CREATE TABLE IF NOT EXISTS revision_price_overrides (
    id BIGSERIAL PRIMARY KEY,
    locale VARCHAR(2) NOT NULL,
    revision_id BIGINT NOT NULL,
    old_cost BIGINT NOT NULL,
    new_cost BIGINT NOT NULL,
    changed_by VARCHAR(255) NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (locale, revision_id) REFERENCES revisions (locale, revision_id) ON DELETE CASCADE
);

ALTER TABLE revisions ADD COLUMN cost_overridden BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS revision_price_overrides_revision_idx ON revision_price_overrides (locale, revision_id);

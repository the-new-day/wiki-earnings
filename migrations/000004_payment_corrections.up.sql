CREATE TABLE IF NOT EXISTS editor_payment_corrections (
    correction_id SERIAL PRIMARY KEY,
    editor_id INT NOT NULL REFERENCES editors(editor_id) ON DELETE CASCADE,
    correction_amount INT NOT NULL,
    description TEXT NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    -- created_at is audit only: when the row was entered.
    -- applies_at is what a monthly report buckets on: the first instant of the
    -- month the correction belongs to, which an admin may backdate.
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    applies_at TIMESTAMPTZ NOT NULL DEFAULT (date_trunc('month', now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')
);

CREATE INDEX IF NOT EXISTS editor_payment_corrections_editor_applies_at_idx
    ON editor_payment_corrections (editor_id, applies_at);

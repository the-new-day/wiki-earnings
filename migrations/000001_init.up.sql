CREATE TABLE IF NOT EXISTS editors (
    editor_id SERIAL PRIMARY KEY,
    nickname VARCHAR(255) UNIQUE NOT NULL
);

-- A wiki account is per-locale: the same person has a separate userid on every
-- language wiki, and all of them are paid out as one editor.
CREATE TABLE IF NOT EXISTS editor_accounts (
    wiki_id INT NOT NULL,
    locale VARCHAR(2) NOT NULL,
    editor_id INT NOT NULL REFERENCES editors (editor_id) ON DELETE CASCADE,
    PRIMARY KEY (locale, wiki_id)
);

CREATE INDEX IF NOT EXISTS editor_accounts_editor_id_idx ON editor_accounts (editor_id);

-- Revision ids are only unique within one wiki, hence the composite key.
CREATE TABLE IF NOT EXISTS revisions (
    revision_id BIGINT NOT NULL,
    locale VARCHAR(2) NOT NULL,
    editor_id INT NOT NULL REFERENCES editors (editor_id) ON DELETE CASCADE,
    page_id BIGINT NOT NULL,
    page_title VARCHAR(255) NOT NULL,
    type SMALLINT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    cost BIGINT NOT NULL,
    edited_at TIMESTAMPTZ NOT NULL,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (locale, revision_id)
);

-- Earnings are always read as "this editor, over this period".
CREATE INDEX IF NOT EXISTS revisions_editor_edited_at_idx ON revisions (editor_id, edited_at);
CREATE INDEX IF NOT EXISTS revisions_edited_at_idx ON revisions (edited_at);

-- Dead letter: revisions that could not be priced. The editor is kept as raw
-- wiki identity rather than a foreign key, because "editor is unknown yet" is
-- one of the reasons a revision lands here.
CREATE TABLE IF NOT EXISTS failed_revisions (
    revision_id BIGINT NOT NULL,
    locale VARCHAR(2) NOT NULL,
    page_id BIGINT NOT NULL,
    page_title VARCHAR(255) NOT NULL,
    wiki_user_id INT NOT NULL,
    nickname VARCHAR(255) NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    type SMALLINT NOT NULL,
    edited_at TIMESTAMPTZ NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    last_attempt_at TIMESTAMPTZ,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (locale, revision_id),
    CONSTRAINT failed_revisions_status_check CHECK (status IN ('pending', 'permanent'))
);

-- The replayer picks up the oldest pending ones in batches.
CREATE INDEX IF NOT EXISTS failed_revisions_pending_idx
    ON failed_revisions (last_attempt_at NULLS FIRST)
    WHERE status = 'pending';

-- How far the sync has got through each wiki's recent changes.
CREATE TABLE IF NOT EXISTS sync_state (
    locale VARCHAR(2) PRIMARY KEY,
    last_rev_id BIGINT NOT NULL DEFAULT 0,
    last_edited_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

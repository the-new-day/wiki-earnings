package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
	"github.com/the-new-day/protanki-wiki-admin/internal/sync"
	"github.com/the-new-day/protanki-wiki-admin/internal/usecase/resync"
)

var (
	_ sync.DeadLetter   = (*DeadLetterRepository)(nil)
	_ resync.DeadLetter = (*DeadLetterRepository)(nil)
)

type DeadLetterRepository struct {
	pool *pgxpool.Pool
}

func NewDeadLetterRepository(pool *pgxpool.Pool) *DeadLetterRepository {
	return &DeadLetterRepository{
		pool: pool,
	}
}

// Put parks a revision. A replayed batch parks the same failure twice, so a
// conflict on the first attempt's row is left untouched rather than overwritten.
func (repo *DeadLetterRepository) Put(ctx context.Context, f entity.FailedRevision) error {
	_, err := repo.pool.Exec(ctx, `
		INSERT INTO failed_revisions
			(revision_id, locale, page_id, page_title, wiki_user_id, nickname, comment, type,
			 edited_at, attempts, last_error, last_attempt_at, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (locale, revision_id) DO NOTHING`,
		f.RevID, f.Locale, f.PageID, f.PageTitle, f.WikiUserID, f.Nickname, f.Comment, f.Type,
		f.EditedAt, f.Attempts, f.LastError, f.LastAttemptAt, f.Status)
	if err != nil {
		return fmt.Errorf("postgres: dead letter %s/%d: %w", f.Locale, f.RevID, err)
	}

	return nil
}

// ListPending returns entries waiting for another attempt, oldest attempt first.
func (repo *DeadLetterRepository) ListPending(ctx context.Context, limit int) ([]entity.FailedRevision, error) {
	rows, err := repo.pool.Query(ctx, `
		SELECT revision_id, locale, page_id, page_title, wiki_user_id, nickname, comment, type,
			edited_at, attempts, last_error, last_attempt_at, status, created_at
		FROM failed_revisions
		WHERE status = 'pending'
		ORDER BY last_attempt_at NULLS FIRST
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: list pending: %w", err)
	}
	defer rows.Close()

	var out []entity.FailedRevision
	for rows.Next() {
		var f entity.FailedRevision
		err := rows.Scan(&f.RevID, &f.Locale, &f.PageID, &f.PageTitle, &f.WikiUserID, &f.Nickname,
			&f.Comment, &f.Type, &f.EditedAt, &f.Attempts, &f.LastError, &f.LastAttemptAt, &f.Status, &f.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("postgres: list pending: scan: %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list pending: %w", err)
	}

	return out, nil
}

// Fail records another failed attempt and leaves the entry pending.
func (repo *DeadLetterRepository) Fail(ctx context.Context, locale string, revID int64, reason string) error {
	_, err := repo.pool.Exec(ctx, `
		UPDATE failed_revisions
		SET attempts = attempts + 1, last_error = $1, last_attempt_at = now()
		WHERE locale = $2 AND revision_id = $3`, reason, locale, revID)
	if err != nil {
		return fmt.Errorf("postgres: fail %s/%d: %w", locale, revID, err)
	}

	return nil
}

// Retire stops retrying an entry for good.
func (repo *DeadLetterRepository) Retire(ctx context.Context, locale string, revID int64, reason string) error {
	_, err := repo.pool.Exec(ctx, `
		UPDATE failed_revisions
		SET status = 'permanent', last_error = $1, last_attempt_at = now()
		WHERE locale = $2 AND revision_id = $3`, reason, locale, revID)
	if err != nil {
		return fmt.Errorf("postgres: retire %s/%d: %w", locale, revID, err)
	}

	return nil
}

// Clear drops every dead-lettered entry for a locale, so a resync starts with
// a clean slate instead of carrying forward stale failure records.
func (repo *DeadLetterRepository) Clear(ctx context.Context, locale string) error {
	_, err := repo.pool.Exec(ctx, `DELETE FROM failed_revisions WHERE locale = $1`, locale)
	if err != nil {
		return fmt.Errorf("postgres: clear dead letters %s: %w", locale, err)
	}

	return nil
}

// Resolve removes an entry that finally went through.
func (repo *DeadLetterRepository) Resolve(ctx context.Context, locale string, revID int64) error {
	_, err := repo.pool.Exec(ctx, `
		DELETE FROM failed_revisions WHERE locale = $1 AND revision_id = $2`, locale, revID)
	if err != nil {
		return fmt.Errorf("postgres: resolve %s/%d: %w", locale, revID, err)
	}

	return nil
}

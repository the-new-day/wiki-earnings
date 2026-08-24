package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
	"github.com/the-new-day/wiki-earnings/internal/storage"
	"github.com/the-new-day/wiki-earnings/internal/sync"
	"github.com/the-new-day/wiki-earnings/internal/usecase/earnings"
	"github.com/the-new-day/wiki-earnings/internal/usecase/revisions"
)

var (
	_ sync.RevisionWriter     = (*RevisionRepository)(nil)
	_ earnings.RevisionReader = (*RevisionRepository)(nil)
	_ revisions.RevisionStore = (*RevisionRepository)(nil)
)

type RevisionRepository struct {
	pool *pgxpool.Pool
}

func NewRevisionRepository(pool *pgxpool.Pool) *RevisionRepository {
	return &RevisionRepository{
		pool: pool,
	}
}

// Upsert stores a priced revision, overwriting whatever was there. A replayed
// batch re-prices the same revision, so this has to accept that without
// erroring. cost is the exception: once an admin has overridden it, a fresh
// computed price must not silently replace it.
// TODO: move cost_overridden logic to usecase, it shouldn't be the repo's responsibility.
func (repo *RevisionRepository) Upsert(ctx context.Context, r entity.Revision) error {
	_, err := repo.pool.Exec(ctx, `
		INSERT INTO revisions (revision_id, locale, editor_id, page_id, page_title, type, comment, cost, edited_at, computed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (locale, revision_id) DO UPDATE SET
			editor_id = EXCLUDED.editor_id,
			page_id = EXCLUDED.page_id,
			page_title = EXCLUDED.page_title,
			type = EXCLUDED.type,
			comment = EXCLUDED.comment,
			cost = CASE WHEN revisions.cost_overridden THEN revisions.cost ELSE EXCLUDED.cost END,
			edited_at = EXCLUDED.edited_at,
			computed_at = EXCLUDED.computed_at`,
		r.RevID, r.Locale, r.EditorID, r.PageID, r.PageTitle, r.Type, r.Comment, r.Cost, r.EditedAt, r.ComputedAt)
	if err != nil {
		return fmt.Errorf("postgres: upsert revision %s/%d: %w", r.Locale, r.RevID, err)
	}

	return nil
}

// SumCostByEditor totals cost per editor over a half-open period [from, to).
func (repo *RevisionRepository) SumCostByEditor(ctx context.Context, from, to time.Time) ([]entity.EditorEarnings, error) {
	rows, err := repo.pool.Query(ctx, `
		SELECT e.editor_id, e.nickname, COALESCE(e.payments_nickname, ''), COALESCE(SUM(r.cost), 0)
		FROM revisions r
		JOIN editors e ON e.editor_id = r.editor_id
		WHERE r.edited_at >= $1 AND r.edited_at < $2
		GROUP BY e.editor_id, e.nickname, e.payments_nickname
		ORDER BY e.editor_id`, from, to)
	if err != nil {
		return nil, fmt.Errorf("postgres: sum cost by editor: %w", err)
	}
	defer rows.Close()

	var out []entity.EditorEarnings
	for rows.Next() {
		var e entity.EditorEarnings
		if err := rows.Scan(&e.EditorID, &e.Nickname, &e.PaymentsNickname, &e.Total); err != nil {
			return nil, fmt.Errorf("postgres: sum cost by editor: scan: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: sum cost by editor: %w", err)
	}

	return out, nil
}

// SumCostForEditor totals one editor's cost over a half-open period [from, to).
func (repo *RevisionRepository) SumCostForEditor(ctx context.Context, editorID int64, from, to time.Time) (int64, error) {
	var total int64
	err := repo.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(cost), 0)
		FROM revisions
		WHERE editor_id = $1 AND edited_at >= $2 AND edited_at < $3`,
		editorID, from, to).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("postgres: sum cost for editor %d: %w", editorID, err)
	}

	return total, nil
}

// ListByEditor lists one editor's revisions over a half-open period [from, to), oldest first.
func (repo *RevisionRepository) ListByEditor(ctx context.Context, editorID int64, from, to time.Time) ([]entity.Revision, error) {
	rows, err := repo.pool.Query(ctx, `
		SELECT revision_id, locale, editor_id, page_id, page_title, type, comment, cost, edited_at, computed_at, cost_overridden
		FROM revisions
		WHERE editor_id = $1 AND edited_at >= $2 AND edited_at < $3
		ORDER BY edited_at`, editorID, from, to)
	if err != nil {
		return nil, fmt.Errorf("postgres: list revisions for editor %d: %w", editorID, err)
	}
	defer rows.Close()

	var out []entity.Revision
	for rows.Next() {
		var r entity.Revision
		err := rows.Scan(&r.RevID, &r.Locale, &r.EditorID, &r.PageID, &r.PageTitle,
			&r.Type, &r.Comment, &r.Cost, &r.EditedAt, &r.ComputedAt, &r.CostOverridden)
		if err != nil {
			return nil, fmt.Errorf("postgres: list revisions for editor %d: scan: %w", editorID, err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list revisions for editor %d: %w", editorID, err)
	}

	return out, nil
}

// Get looks up a single revision.
func (repo *RevisionRepository) Get(ctx context.Context, locale string, revisionID int64) (entity.Revision, error) {
	var r entity.Revision
	err := repo.pool.QueryRow(ctx, `
		SELECT revision_id, locale, editor_id, page_id, page_title, type, comment, cost, edited_at, computed_at, cost_overridden
		FROM revisions WHERE locale = $1 AND revision_id = $2`, locale, revisionID).
		Scan(&r.RevID, &r.Locale, &r.EditorID, &r.PageID, &r.PageTitle, &r.Type, &r.Comment, &r.Cost, &r.EditedAt, &r.ComputedAt, &r.CostOverridden)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Revision{}, fmt.Errorf("postgres: get revision %s/%d: %w", locale, revisionID, storage.ErrNotFound)
	}
	if err != nil {
		return entity.Revision{}, fmt.Errorf("postgres: get revision %s/%d: %w", locale, revisionID, err)
	}

	return r, nil
}

// OverrideCost sets a revision's price by hand and logs the change: who made
// it, when, and what the price was before. A later resync re-prices and
// overwrites the row in revisions, but the log entry stands regardless.
func (repo *RevisionRepository) OverrideCost(ctx context.Context, locale string, revisionID int64, newCost int64, changedBy string) (entity.Revision, error) {
	var r entity.Revision
	var oldCost int64

	err := pgx.BeginFunc(ctx, repo.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT revision_id, locale, editor_id, page_id, page_title, type, comment, cost, edited_at, computed_at
			FROM revisions WHERE locale = $1 AND revision_id = $2 FOR UPDATE`, locale, revisionID).
			Scan(&r.RevID, &r.Locale, &r.EditorID, &r.PageID, &r.PageTitle, &r.Type, &r.Comment, &oldCost, &r.EditedAt, &r.ComputedAt)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			UPDATE revisions SET cost = $1, cost_overridden = true WHERE locale = $2 AND revision_id = $3`,
			newCost, locale, revisionID); err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO revision_price_overrides (locale, revision_id, old_cost, new_cost, changed_by)
			VALUES ($1, $2, $3, $4, $5)`, locale, revisionID, oldCost, newCost, changedBy)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Revision{}, fmt.Errorf("postgres: override cost %s/%d: %w", locale, revisionID, storage.ErrNotFound)
	}
	if err != nil {
		return entity.Revision{}, fmt.Errorf("postgres: override cost %s/%d: %w", locale, revisionID, err)
	}

	r.Cost = newCost
	r.CostOverridden = true
	return r, nil
}

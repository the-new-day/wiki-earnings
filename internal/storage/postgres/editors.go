package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
	"github.com/the-new-day/wiki-earnings/internal/storage"
	"github.com/the-new-day/wiki-earnings/internal/sync"
	"github.com/the-new-day/wiki-earnings/internal/usecase/earnings"
	"github.com/the-new-day/wiki-earnings/internal/usecase/editors"
	"github.com/the-new-day/wiki-earnings/internal/usecase/revisions"
)

var (
	_ sync.EditorRegistry    = (*EditorRepository)(nil)
	_ earnings.EditorReader  = (*EditorRepository)(nil)
	_ revisions.EditorReader = (*EditorRepository)(nil)
	_ editors.Repository     = (*EditorRepository)(nil)
)

type EditorRepository struct {
	pool *pgxpool.Pool
}

func NewEditorRepository(pool *pgxpool.Pool) *EditorRepository {
	return &EditorRepository{
		pool: pool,
	}
}

func (repo *EditorRepository) GetByID(ctx context.Context, id int64) (entity.Editor, error) {
	var e entity.Editor
	err := repo.pool.QueryRow(ctx, `
		SELECT editor_id, nickname, COALESCE(payments_nickname, '')
		FROM editors WHERE editor_id = $1`, id).
		Scan(&e.EditorID, &e.Nickname, &e.PaymentsNickname)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Editor{}, fmt.Errorf("postgres: get editor %d: %w", id, storage.ErrNotFound)
	}
	if err != nil {
		return entity.Editor{}, fmt.Errorf("postgres: get editor %d: %w", id, err)
	}

	return e, nil
}

func (repo *EditorRepository) FindByNickname(ctx context.Context, nickname string) (entity.Editor, error) {
	var e entity.Editor
	err := repo.pool.QueryRow(ctx, `
		SELECT editor_id, nickname, COALESCE(payments_nickname, '')
		FROM editors WHERE nickname ILIKE $1`, nickname).
		Scan(&e.EditorID, &e.Nickname, &e.PaymentsNickname)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Editor{}, fmt.Errorf("postgres: find editor %q: %w", nickname, storage.ErrNotFound)
	}
	if err != nil {
		return entity.Editor{}, fmt.Errorf("postgres: find editor %q: %w", nickname, err)
	}

	return e, nil
}

// FindByLocaleUser looks up the editor behind a wiki account. A miss is not an
// error -- it means the account has not been seen before.
func (repo *EditorRepository) FindByLocaleUser(ctx context.Context, locale string, wikiUserID int64) (entity.Editor, bool, error) {
	var e entity.Editor
	err := repo.pool.QueryRow(ctx, `
		SELECT e.editor_id, e.nickname, COALESCE(e.payments_nickname, '')
		FROM editor_accounts ea
		JOIN editors e ON e.editor_id = ea.editor_id
		WHERE ea.locale = $1 AND ea.wiki_id = $2`, locale, wikiUserID).
		Scan(&e.EditorID, &e.Nickname, &e.PaymentsNickname)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Editor{}, false, nil
	}
	if err != nil {
		return entity.Editor{}, false, fmt.Errorf("postgres: find editor for %s/%d: %w", locale, wikiUserID, err)
	}

	return e, true, nil
}

// Register records a wiki account and the editor behind it. It converges to
// the same editor under concurrent duplicate calls: the nickname upsert always
// resolves to the same row because nickname is unique, and the account link is
// a no-op the second time round.
func (repo *EditorRepository) Register(ctx context.Context, locale string, wikiUserID int64, nickname string) (entity.Editor, error) {
	var editorID int64

	err := pgx.BeginFunc(ctx, repo.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO editors (nickname) VALUES ($1)
			ON CONFLICT (nickname) DO UPDATE SET nickname = EXCLUDED.nickname
			RETURNING editor_id`, nickname).Scan(&editorID)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO editor_accounts (locale, wiki_id, editor_id) VALUES ($1, $2, $3)
			ON CONFLICT (locale, wiki_id) DO NOTHING`, locale, wikiUserID, editorID)
		return err
	})
	if err != nil {
		return entity.Editor{}, fmt.Errorf("postgres: register %s/%d (%q): %w", locale, wikiUserID, nickname, err)
	}

	return entity.Editor{EditorID: editorID, Nickname: nickname}, nil
}

// SetPaymentsNickname sets the account an editor is paid on.
// An empty nickname clears it.
func (repo *EditorRepository) SetPaymentsNickname(ctx context.Context, editorID int64, nickname string) error {
	var stored *string
	if nickname != "" {
		stored = &nickname
	}

	tag, err := repo.pool.Exec(ctx, `
		UPDATE editors SET payments_nickname = $1 WHERE editor_id = $2`, stored, editorID)
	if err != nil {
		return fmt.Errorf("postgres: set payments nickname of editor %d to %q: %w", editorID, nickname, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: set payments nickname of editor %d: %w", editorID, storage.ErrNotFound)
	}

	return nil
}

func (repo *EditorRepository) Rename(ctx context.Context, editorID int64, nickname string) error {
	_, err := repo.pool.Exec(ctx, `
		UPDATE editors SET nickname = $1 WHERE editor_id = $2`, nickname, editorID)
	if err != nil {
		return fmt.Errorf("postgres: rename editor %d to %q: %w", editorID, nickname, err)
	}

	return nil
}

// Locales lists the wikis an editor has an account on.
func (repo *EditorRepository) Locales(ctx context.Context, editorID int64) ([]string, error) {
	rows, err := repo.pool.Query(ctx, `
		SELECT locale FROM editor_accounts WHERE editor_id = $1 ORDER BY locale`, editorID)
	if err != nil {
		return nil, fmt.Errorf("postgres: locales for editor %d: %w", editorID, err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var locale string
		if err := rows.Scan(&locale); err != nil {
			return nil, fmt.Errorf("postgres: locales for editor %d: scan: %w", editorID, err)
		}
		out = append(out, locale)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: locales for editor %d: %w", editorID, err)
	}

	return out, nil
}

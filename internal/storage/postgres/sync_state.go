package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
	"github.com/the-new-day/protanki-wiki-admin/internal/sync"
)

var _ sync.StateStore = (*SyncStateRepository)(nil)

type SyncStateRepository struct {
	pool *pgxpool.Pool
}

func NewSyncStateRepository(pool *pgxpool.Pool) *SyncStateRepository {
	return &SyncStateRepository{
		pool: pool,
	}
}

// Get returns the zero SyncState for a locale that has never been synced.
func (repo *SyncStateRepository) Get(ctx context.Context, locale string) (entity.SyncState, error) {
	var s entity.SyncState
	err := repo.pool.QueryRow(ctx, `
		SELECT locale, last_rev_id, last_edited_at, updated_at
		FROM sync_state WHERE locale = $1`, locale).
		Scan(&s.Locale, &s.LastRevID, &s.LastEditedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.SyncState{Locale: locale}, nil
	}
	if err != nil {
		return entity.SyncState{}, fmt.Errorf("postgres: get sync state %s: %w", locale, err)
	}

	return s, nil
}

func (repo *SyncStateRepository) Save(ctx context.Context, s entity.SyncState) error {
	_, err := repo.pool.Exec(ctx, `
		INSERT INTO sync_state (locale, last_rev_id, last_edited_at, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (locale) DO UPDATE SET
			last_rev_id = EXCLUDED.last_rev_id,
			last_edited_at = EXCLUDED.last_edited_at,
			updated_at = EXCLUDED.updated_at`,
		s.Locale, s.LastRevID, s.LastEditedAt, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres: save sync state %s: %w", s.Locale, err)
	}

	return nil
}

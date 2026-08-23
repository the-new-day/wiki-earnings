package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/the-new-day/wiki-earnings/internal/config"
	"github.com/the-new-day/wiki-earnings/internal/domain/pricing"
	"github.com/the-new-day/wiki-earnings/internal/mediawiki"
	"github.com/the-new-day/wiki-earnings/internal/storage/postgres"
	"github.com/the-new-day/wiki-earnings/internal/sync"
	"github.com/the-new-day/wiki-earnings/internal/usecase/earnings"
	"github.com/the-new-day/wiki-earnings/internal/usecase/resync"
	"github.com/the-new-day/wiki-earnings/internal/usecase/revisions"
)

type App struct {
	Sync      *sync.Service
	Earnings  *earnings.UseCase
	Revisions *revisions.UseCase
	Resync    *resync.UseCase

	pool *pgxpool.Pool
}

// New opens the database and builds everything above it. The caller owns the
// result and must Close it.
func New(ctx context.Context, cfg config.Config) (*App, error) {
	pool, err := postgres.Connect(ctx, cfg.Postgres)
	if err != nil {
		return nil, err
	}

	editors := postgres.NewEditorRepository(pool)
	revisionRepo := postgres.NewRevisionRepository(pool)
	syncState := postgres.NewSyncStateRepository(pool)
	deadLetter := postgres.NewDeadLetterRepository(pool)

	syncSvc := sync.New(
		mediawiki.NewClient(),
		editors,
		revisionRepo,
		syncState,
		deadLetter,
		pricing.Default(),
		postgres.NewLocker(pool),
		sync.Config{
			Locales:         cfg.Locales,
			BatchSize:       cfg.SyncBatchSize,
			InitialLookback: cfg.InitialLookback,
			MinInterval:     cfg.SyncMinInterval,
			MaxDuration:     cfg.SyncMaxDuration,
			Concurrency:     cfg.SyncConcurrency,
			MaxAttempts:     cfg.DeadLetterMaxAttempts,
			ReplayBatchSize: cfg.DeadLetterBatchSize,
		},
	)

	return &App{
		Sync:      syncSvc,
		Earnings:  earnings.New(editors, revisionRepo, syncSvc),
		Revisions: revisions.New(editors, revisionRepo),
		Resync:    resync.New(syncState, deadLetter, syncSvc, cfg.Locales),
		pool:      pool,
	}, nil
}

// Close releases the database pool.
// Everything built on it is unusable afterwards.
func (a *App) Close() {
	a.pool.Close()
}

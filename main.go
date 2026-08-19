package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/the-new-day/protanki-wiki-admin/internal/config"
	"github.com/the-new-day/protanki-wiki-admin/internal/domain/pricing"
	"github.com/the-new-day/protanki-wiki-admin/internal/mediawiki"
	"github.com/the-new-day/protanki-wiki-admin/internal/storage/postgres"
	"github.com/the-new-day/protanki-wiki-admin/internal/sync"
	"github.com/the-new-day/protanki-wiki-admin/internal/usecase/earnings"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := postgres.Connect(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer pool.Close()

	log.Printf("connected to postgres %s:%d/%s, locales: %v",
		cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.Database, cfg.Locales)

	editorRepo := postgres.NewEditorRepository(pool)
	revisionRepo := postgres.NewRevisionRepository(pool)
	syncStateRepo := postgres.NewSyncStateRepository(pool)
	deadLetterRepo := postgres.NewDeadLetterRepository(pool)
	locker := postgres.NewLocker(pool)

	syncSvc := sync.New(
		mediawiki.NewClient(),
		editorRepo,
		revisionRepo,
		syncStateRepo,
		deadLetterRepo,
		pricing.Default(),
		locker,
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

	earningsUC := earnings.New(editorRepo, revisionRepo, syncSvc)
	_ = earningsUC // TODO: wire into HTTP handlers once the transport layer exists

	// TODO: run syncSvc.Replay on a ticker once the transport layer exists.
	<-ctx.Done()
	log.Println("shutting down")

	return nil
}

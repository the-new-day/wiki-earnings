package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/the-new-day/protanki-wiki-admin/internal/config"
	"github.com/the-new-day/protanki-wiki-admin/internal/delivery/discord"
	"github.com/the-new-day/protanki-wiki-admin/internal/domain/pricing"
	"github.com/the-new-day/protanki-wiki-admin/internal/mediawiki"
	"github.com/the-new-day/protanki-wiki-admin/internal/storage/postgres"
	"github.com/the-new-day/protanki-wiki-admin/internal/sync"
	"github.com/the-new-day/protanki-wiki-admin/internal/usecase/earnings"
	"github.com/the-new-day/protanki-wiki-admin/internal/usecase/resync"
	"github.com/the-new-day/protanki-wiki-admin/internal/usecase/revisions"
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
	revisionsUC := revisions.New(editorRepo, revisionRepo)
	resyncUC := resync.New(syncStateRepo, deadLetterRepo, syncSvc, cfg.Locales)

	bot, err := discord.New(
		cfg.DiscordBotToken,
		earningsUC,
		revisionsUC,
		resyncUC,
		cfg.WikiRoleID,
		cfg.WikiAdminRoleID,
		cfg.MessageLifetime,
	)
	if err != nil {
		return err
	}

	replayDone := make(chan struct{})
	go runReplayLoop(ctx, syncSvc, cfg.ReplayInterval, replayDone)

	botDone := make(chan error, 1)
	go func() {
		botDone <- bot.Run(ctx)
	}()

	<-ctx.Done()
	log.Println("shutting down")
	<-replayDone

	if err := <-botDone; err != nil {
		return err
	}

	return nil
}

// runReplayLoop retries dead-lettered revisions on a schedule.
func runReplayLoop(ctx context.Context, syncSvc *sync.Service, interval time.Duration, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := syncSvc.Replay(ctx); err != nil {
				log.Printf("replay: %v", err)
			}
		}
	}
}

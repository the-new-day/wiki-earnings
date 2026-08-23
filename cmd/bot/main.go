package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/the-new-day/wiki-earnings/internal/app"
	"github.com/the-new-day/wiki-earnings/internal/config"
	"github.com/the-new-day/wiki-earnings/internal/delivery/discord"
	"github.com/the-new-day/wiki-earnings/internal/sync"
	"github.com/the-new-day/wiki-earnings/internal/translate"
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

	a, err := app.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer a.Close()

	log.Printf("connected to postgres %s:%d/%s, locales: %v",
		cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.Database, cfg.Locales)

	bot, err := discord.New(
		cfg.Discord.BotToken,
		a.Earnings,
		a.Revisions,
		a.Resync,
		cfg.Discord.WikiRoleID,
		cfg.Discord.WikiAdminRoleID,
		cfg.MessageLifetime,
		discord.TaskConfig{
			Translator: translate.NewPassthrough(),
			Languages:  cfg.TaskLanguages(),
			Channels:   cfg.TaskChannels(),
		},
	)
	if err != nil {
		return err
	}

	replayDone := make(chan struct{})
	go runReplayLoop(ctx, a.Sync, cfg.ReplayInterval, replayDone)

	botDone := make(chan error, 1)
	go func() {
		botDone <- bot.Run(ctx)
	}()

	var botErr error
	botExited := false

	select {
	case <-ctx.Done():
	case botErr = <-botDone:
		botExited = true
		stop()
	}

	log.Println("shutting down")
	<-replayDone

	if !botExited {
		botErr = <-botDone
	}

	return botErr
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

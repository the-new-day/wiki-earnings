package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/the-new-day/protanki-wiki-admin/internal/config"
	"github.com/the-new-day/protanki-wiki-admin/internal/storage/postgres"
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

	// TODO: repositories over pool, then the sync loop.
	<-ctx.Done()
	log.Println("shutting down")

	return nil
}

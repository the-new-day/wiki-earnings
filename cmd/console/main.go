package main

import (
	"context"
	"log"
	"os"

	"github.com/the-new-day/wiki-earnings/internal/app"
	"github.com/the-new-day/wiki-earnings/internal/config"
	"github.com/the-new-day/wiki-earnings/internal/delivery/console"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	a, err := app.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer a.Close()

	return console.New(a.Earnings, a.Revisions, a.Resync, a.Sync).Run(ctx, os.Stdin, os.Stdout)
}

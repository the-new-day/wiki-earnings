// Command seed fills a development database with editors to price revisions
// against. It is idempotent and refuses to touch anything but a dev database.
package main

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/the-new-day/protanki-wiki-admin/internal/config"
	"github.com/the-new-day/protanki-wiki-admin/internal/storage/postgres"
)

type account struct {
	locale string
	wikiID int64
}

type editor struct {
	nickname string
	accounts []account
}

// Replace with whatever your local wiki accounts actually are.
var editors = []editor{
	{nickname: "New.Day", accounts: []account{{"ru", 1}}},
	{nickname: "Tester", accounts: []account{{"ru", 2}, {"en", 2}}},
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	pool, err := postgres.Connect(ctx, cfg.Postgres)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		for _, e := range editors {
			var editorID int64

			err := tx.QueryRow(ctx, `
				INSERT INTO editors (nickname) VALUES ($1)
				ON CONFLICT (nickname) DO UPDATE SET nickname = EXCLUDED.nickname
				RETURNING editor_id`, e.nickname).Scan(&editorID)
			if err != nil {
				return err
			}

			for _, a := range e.accounts {
				_, err := tx.Exec(ctx, `
					INSERT INTO editor_accounts (wiki_id, locale, editor_id) VALUES ($1, $2, $3)
					ON CONFLICT (locale, wiki_id) DO UPDATE SET editor_id = EXCLUDED.editor_id`,
					a.wikiID, a.locale, editorID)
				if err != nil {
					return err
				}
			}

			log.Printf("seeded editor %q (id=%d, %d accounts)", e.nickname, editorID, len(e.accounts))
		}

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Println("seed complete")
}

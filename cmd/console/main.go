package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/the-new-day/protanki-wiki-admin/internal/config"
	"github.com/the-new-day/protanki-wiki-admin/internal/domain/pricing"
	"github.com/the-new-day/protanki-wiki-admin/internal/mediawiki"
	"github.com/the-new-day/protanki-wiki-admin/internal/storage/postgres"
	"github.com/the-new-day/protanki-wiki-admin/internal/sync"
	"github.com/the-new-day/protanki-wiki-admin/internal/usecase/earnings"
	"github.com/the-new-day/protanki-wiki-admin/internal/usecase/resync"
	"github.com/the-new-day/protanki-wiki-admin/internal/usecase/revisions"
)

const monthLayout = "2006-01"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	loadDotEnv(".env")

	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := postgres.Connect(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer pool.Close()

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

	fmt.Println("connected. commands:")
	fmt.Println("  salary <nickname> [month=YYYY-MM]   (syncs first)")
	fmt.Println("  edits <nickname> [month=YYYY-MM]    (syncs first)")
	fmt.Println("  report [month=YYYY-MM]              (syncs first)")
	fmt.Println("  changepay <nickname> <edit_id> <new_cost> [locale]")
	fmt.Println("  sync                                (run sync directly)")
	fmt.Println("  replay                              (retry dead-lettered revisions)")
	fmt.Println("  resync                              (wipe sync state + dead letters, sync from scratch)")
	fmt.Println("  quit")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			return scanner.Err()
		}

		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}

		cmd, args := fields[0], fields[1:]
		if cmd == "quit" || cmd == "exit" {
			return nil
		}

		result, err := dispatch(ctx, earningsUC, revisionsUC, resyncUC, syncSvc, cmd, args)
		if err != nil {
			fmt.Println("error:", err)
			continue
		}

		printJSON(result)
	}
}

func dispatch(
	ctx context.Context,
	earningsUC *earnings.UseCase,
	revisionsUC *revisions.UseCase,
	resyncUC *resync.UseCase,
	syncSvc *sync.Service,
	cmd string,
	args []string,
) (any, error) {
	switch cmd {
	case "sync":
		if err := syncSvc.Sync(ctx); err != nil {
			return nil, err
		}

		return "sync ok", nil

	case "replay":
		if err := syncSvc.Replay(ctx); err != nil {
			return nil, err
		}

		return "replay ok", nil

	case "resync":
		if err := resyncUC.Resync(ctx); err != nil {
			return nil, err
		}

		return "resync ok", nil

	case "salary":
		if len(args) < 1 {
			return nil, fmt.Errorf("usage: salary <nickname> [month]")
		}

		from, to, err := monthRange(arg(args, 1))
		if err != nil {
			return nil, err
		}

		payslip, err := earningsUC.ForNickname(ctx, args[0], from, to)
		if err != nil {
			return nil, err
		}

		return struct {
			Editor any       `json:"editor"`
			From   time.Time `json:"from"`
			To     time.Time `json:"to"`
			Total  int64     `json:"total"`
		}{payslip.Editor, payslip.From, payslip.To, payslip.Total}, nil

	case "edits":
		if len(args) < 1 {
			return nil, fmt.Errorf("usage: edits <nickname> [month]")
		}

		from, to, err := monthRange(arg(args, 1))
		if err != nil {
			return nil, err
		}

		return earningsUC.ForNickname(ctx, args[0], from, to)

	case "report":
		from, to, err := monthRange(arg(args, 0))
		if err != nil {
			return nil, err
		}

		return earningsUC.Report(ctx, from, to)

	case "changepay":
		if len(args) < 3 {
			return nil, fmt.Errorf("usage: changepay <nickname> <edit_id> <new_cost> [locale]")
		}

		editID, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("edit_id: %w", err)
		}

		newCost, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("new_cost: %w", err)
		}

		return revisionsUC.OverridePrice(ctx, args[0], arg(args, 3), editID, newCost, "console")

	default:
		return nil, fmt.Errorf("unknown command %q", cmd)
	}
}

// arg returns args[i], or "" if there aren't that many.
func arg(args []string, i int) string {
	if i >= len(args) {
		return ""
	}

	return args[i]
}

// monthRange parses "YYYY-MM" into a half-open [from, to) period covering
// that month. An empty raw defaults to the current month.
func monthRange(raw string) (time.Time, time.Time, error) {
	var from time.Time

	if raw == "" {
		now := time.Now().UTC()
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	} else {
		parsed, err := time.Parse(monthLayout, raw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("month %q: expected YYYY-MM: %w", raw, err)
		}
		from = time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, time.UTC)
	}

	return from, from.AddDate(0, 1, 0), nil
}

func printJSON(v any) {
	encoded, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println("error marshalling result:", err)
		return
	}

	fmt.Println(string(encoded))
}

// loadDotEnv sets env vars from a .env file for local runs outside docker
// compose. Real env vars always win; a missing file is not an error.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		if _, set := os.LookupEnv(key); set {
			continue
		}

		os.Setenv(key, strings.TrimSpace(value))
	}
}

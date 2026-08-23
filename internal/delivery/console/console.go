// Package console is a REPL over the same use cases the Discord bot serves.
// It exists to look at the pipeline without Discord in the way: results are
// printed as raw JSON rather than formatted for humans.
package console

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/the-new-day/wiki-earnings/internal/usecase/earnings"
	"github.com/the-new-day/wiki-earnings/internal/usecase/resync"
	"github.com/the-new-day/wiki-earnings/internal/usecase/revisions"
)

// Syncer runs the pipeline on demand. The bot only ever syncs as a side effect
// of a read; here it is something to trigger on purpose.
type Syncer interface {
	Sync(ctx context.Context) error
	Replay(ctx context.Context) error
}

const usage = `commands:
  salary <nickname> [month=YYYY-MM]   (syncs first)
  edits <nickname> [month=YYYY-MM]    (syncs first)
  report [month=YYYY-MM]              (syncs first)
  changepay <nickname> <edit_id> <new_cost> [locale]
  sync                                (run sync directly)
  replay                              (retry dead-lettered revisions)
  resync                              (wipe sync state + dead letters, sync from scratch)
  quit`

type Console struct {
	earnings  *earnings.UseCase
	revisions *revisions.UseCase
	resync    *resync.UseCase
	syncer    Syncer
}

func New(
	earningsUC *earnings.UseCase,
	revisionsUC *revisions.UseCase,
	resyncUC *resync.UseCase,
	syncer Syncer,
) *Console {
	return &Console{
		earnings:  earningsUC,
		revisions: revisionsUC,
		resync:    resyncUC,
		syncer:    syncer,
	}
}

// Run reads commands from in until it runs dry or the user quits. A command
// that fails is reported and the loop carries on - only a broken input stream
// ends it.
func (c *Console) Run(ctx context.Context, in io.Reader, out io.Writer) error {
	fmt.Fprintln(out, usage)

	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprint(out, "> ")
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

		result, err := c.dispatch(ctx, cmd, args)
		if err != nil {
			fmt.Fprintln(out, "error:", err)
			continue
		}

		printJSON(out, result)
	}
}

func printJSON(out io.Writer, v any) {
	encoded, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintln(out, "error marshalling result:", err)
		return
	}

	fmt.Fprintln(out, string(encoded))
}

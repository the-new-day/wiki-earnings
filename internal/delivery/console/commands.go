package console

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

const monthLayout = "2006-01"

// changedBy is what a price override made from here is attributed to. The
// console has no user to name.
const changedBy = "console"

// dispatch runs one command and returns whatever should be printed for it.
func (c *Console) dispatch(ctx context.Context, cmd string, args []string) (any, error) {
	switch cmd {
	case "sync":
		if err := c.syncer.Sync(ctx); err != nil {
			return nil, err
		}

		return "sync ok", nil

	case "replay":
		if err := c.syncer.Replay(ctx); err != nil {
			return nil, err
		}

		return "replay ok", nil

	case "resync":
		if err := c.resync.Resync(ctx); err != nil {
			return nil, err
		}

		return "resync ok", nil

	case "salary":
		return c.salary(ctx, args)

	case "edits":
		return c.edits(ctx, args)

	case "report":
		return c.report(ctx, args)

	case "changepay":
		return c.changePay(ctx, args)

	default:
		return nil, fmt.Errorf("unknown command %q", cmd)
	}
}

// salary prints the total only. Use edits for the revisions behind it.
func (c *Console) salary(ctx context.Context, args []string) (any, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("usage: salary <nickname> [month]")
	}

	from, to, err := monthRange(arg(args, 1))
	if err != nil {
		return nil, err
	}

	payslip, err := c.earnings.ForNickname(ctx, args[0], from, to)
	if err != nil {
		return nil, err
	}

	return struct {
		Editor any       `json:"editor"`
		From   time.Time `json:"from"`
		To     time.Time `json:"to"`
		Total  int64     `json:"total"`
	}{payslip.Editor, payslip.From, payslip.To, payslip.Total}, nil
}

func (c *Console) edits(ctx context.Context, args []string) (any, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("usage: edits <nickname> [month]")
	}

	from, to, err := monthRange(arg(args, 1))
	if err != nil {
		return nil, err
	}

	return c.earnings.ForNickname(ctx, args[0], from, to)
}

func (c *Console) report(ctx context.Context, args []string) (any, error) {
	from, to, err := monthRange(arg(args, 0))
	if err != nil {
		return nil, err
	}

	return c.earnings.Report(ctx, from, to)
}

func (c *Console) changePay(ctx context.Context, args []string) (any, error) {
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

	return c.revisions.OverridePrice(ctx, args[0], arg(args, 3), editID, newCost, changedBy)
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

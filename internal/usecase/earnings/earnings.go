package earnings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
)

// ErrEmptyPeriod is returned for a period that cannot contain anything.
// Periods are half-open: from is included, to is not.
var ErrEmptyPeriod = errors.New("earnings: empty period")

// EditorReader looks up who is asking. Repository errors pass through wrapped,
// so callers can still match storage.ErrNotFound with errors.Is.
type EditorReader interface {
	GetByID(ctx context.Context, id int64) (entity.Editor, error)
	FindByNickname(ctx context.Context, nickname string) (entity.Editor, error)
}

// RevisionReader sums already-priced revisions. Pricing happened at sync time;
// nothing here recomputes a cost.
type RevisionReader interface {
	SumCostByEditor(ctx context.Context, from, to time.Time) ([]entity.EditorEarnings, error)
	SumCostForEditor(ctx context.Context, editorID int64, from, to time.Time) (int64, error)
	ListByEditor(ctx context.Context, editorID int64, from, to time.Time) ([]entity.Revision, error)
}

// Syncer brings storage up to date with the wiki. It is called before every
// read so the figures are not stale, but it is not this package's business how
// it does that -- batching, throttling and locking live on the other side.
type Syncer interface {
	Sync(ctx context.Context) error
}

// Payslip is one editor's earnings over a period.
type Payslip struct {
	Editor    entity.Editor
	From      time.Time
	To        time.Time
	Total     int64
	Revisions []entity.Revision

	// SyncErr is set when the sync before this read failed. The figures below
	// are still correct for what is stored -- they may just be missing the
	// newest edits. Show them, with a warning.
	SyncErr error
}

// Report is everyone's earnings over a period.
type Report struct {
	From    time.Time
	To      time.Time
	Editors []entity.EditorEarnings
	Total   int64

	// SyncErr carries the same meaning as on Payslip.
	SyncErr error
}

// UseCase serves earnings queries.
type UseCase struct {
	editors   EditorReader
	revisions RevisionReader
	syncer    Syncer
}

// New wires the use case. A nil syncer means reads are served from whatever is
// already stored, for when something else keeps storage fresh.
func New(editors EditorReader, revisions RevisionReader, syncer Syncer) *UseCase {
	if syncer == nil {
		syncer = noSync{}
	}

	return &UseCase{
		editors:   editors,
		revisions: revisions,
		syncer:    syncer,
	}
}

// ForNickname is the editor-facing entry point: someone types their wiki
// nickname and wants a number back.
func (u *UseCase) ForNickname(ctx context.Context, nickname string, from, to time.Time) (Payslip, error) {
	if err := checkPeriod(from, to); err != nil {
		return Payslip{}, err
	}

	// Before the lookup, not after: an editor whose first edit landed since the
	// last sync has no row yet, and would otherwise be told they do not exist.
	syncErr := u.sync(ctx)

	editor, err := u.editors.FindByNickname(ctx, nickname)
	if err != nil {
		return Payslip{}, fmt.Errorf("earnings: editor %q: %w", nickname, err)
	}

	return u.payslip(ctx, editor, from, to, syncErr)
}

// ForEditor is the same query for a caller that already knows the internal id.
func (u *UseCase) ForEditor(ctx context.Context, editorID int64, from, to time.Time) (Payslip, error) {
	if err := checkPeriod(from, to); err != nil {
		return Payslip{}, err
	}

	syncErr := u.sync(ctx)

	editor, err := u.editors.GetByID(ctx, editorID)
	if err != nil {
		return Payslip{}, fmt.Errorf("earnings: editor %d: %w", editorID, err)
	}

	return u.payslip(ctx, editor, from, to, syncErr)
}

// Report is the payroll view: every editor who earned anything in the period.
func (u *UseCase) Report(ctx context.Context, from, to time.Time) (Report, error) {
	if err := checkPeriod(from, to); err != nil {
		return Report{}, err
	}

	syncErr := u.sync(ctx)

	rows, err := u.revisions.SumCostByEditor(ctx, from, to)
	if err != nil {
		return Report{}, fmt.Errorf("earnings: sum by editor: %w", err)
	}

	report := Report{From: from, To: to, Editors: rows, SyncErr: syncErr}
	for _, r := range rows {
		report.Total += r.Total
	}

	return report, nil
}

func (u *UseCase) payslip(ctx context.Context, editor entity.Editor, from, to time.Time, syncErr error) (Payslip, error) {
	total, err := u.revisions.SumCostForEditor(ctx, editor.EditorID, from, to)
	if err != nil {
		return Payslip{}, fmt.Errorf("earnings: sum for editor %d: %w", editor.EditorID, err)
	}

	revisions, err := u.revisions.ListByEditor(ctx, editor.EditorID, from, to)
	if err != nil {
		return Payslip{}, fmt.Errorf("earnings: list for editor %d: %w", editor.EditorID, err)
	}

	return Payslip{
		Editor:    editor,
		From:      from,
		To:        to,
		Total:     total,
		Revisions: revisions,
		SyncErr:   syncErr,
	}, nil
}

// sync refreshes storage before a read. A failure is deliberately not fatal:
// figures that are a few edits behind, clearly marked, beat an error page.
func (u *UseCase) sync(ctx context.Context) error {
	if err := u.syncer.Sync(ctx); err != nil {
		return fmt.Errorf("earnings: sync: %w", err)
	}

	return nil
}

func checkPeriod(from, to time.Time) error {
	if !to.After(from) {
		return fmt.Errorf("%w: [%s, %s)", ErrEmptyPeriod, from.Format(time.RFC3339), to.Format(time.RFC3339))
	}

	return nil
}

// noSync stands in for an absent Syncer so the read path has no nil checks.
type noSync struct{}

func (noSync) Sync(context.Context) error { return nil }

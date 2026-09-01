package earnings_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
	"github.com/the-new-day/wiki-earnings/internal/usecase/earnings"
	"github.com/the-new-day/wiki-earnings/internal/usecase/earnings/mocks"
)

var (
	errRepo = errors.New("repository is down")
	errWiki = errors.New("wiki unreachable")

	// August 2026, half-open: from is included, to is not.
	from = time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	to   = from.AddDate(0, 1, 0)

	editor = entity.Editor{EditorID: 7, Nickname: "tanker"}

	editorRevisions = []entity.Revision{
		{RevID: 1, Locale: "ru", EditorID: editor.EditorID, Type: entity.ArticleEdit, Cost: 3000},
		{RevID: 2, Locale: "ru", EditorID: editor.EditorID, Type: entity.MinorEdit, Cost: 1200},
	}
)

// deps holds the mocked collaborators. Every mock is built with the *testing.T,
// so an unmet expectation fails the test at cleanup and a call nobody set up
// fails it on the spot.
type deps struct {
	editors     *mocks.MockEditorReader
	revisions   *mocks.MockRevisionReader
	corrections *mocks.MockPaymentCorrectionReader
	syncer      *mocks.MockSyncer
}

func newDeps(t *testing.T) deps {
	t.Helper()

	return deps{
		editors:     mocks.NewMockEditorReader(t),
		revisions:   mocks.NewMockRevisionReader(t),
		corrections: mocks.NewMockPaymentCorrectionReader(t),
		syncer:      mocks.NewMockSyncer(t),
	}
}

func (d deps) useCase() *earnings.UseCase {
	return earnings.New(d.editors, d.revisions, d.corrections, d.syncer)
}

// expectSync sets up the refresh every read starts with.
func (d deps) expectSync(err error) {
	d.syncer.EXPECT().Sync(mock.Anything).Return(err).Once()
}

// expectSums sets up both halves of a payslip read. A failing total short
// circuits, so the listing is only expected when the total succeeds.
func (d deps) expectSums(total int64, revisions []entity.Revision, err error) {
	d.revisions.EXPECT().SumCostForEditor(mock.Anything, editor.EditorID, from, to).Return(total, err).Once()
	if err != nil {
		return
	}

	d.revisions.EXPECT().ListByEditor(mock.Anything, editor.EditorID, from, to).Return(revisions, nil).Once()
	d.corrections.EXPECT().ListByEditor(mock.Anything, editor.EditorID, from, to).Return(nil, nil).Once()
}

func TestUseCase_ForNickname(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(deps)
		wantErr error
		assert  func(*testing.T, earnings.Payslip)
	}{
		{
			name: "pays out what the editor earned",
			arrange: func(d deps) {
				d.expectSync(nil)
				d.editors.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(editor, nil).Once()
				d.expectSums(4200, editorRevisions, nil)
			},
			assert: func(t *testing.T, got earnings.Payslip) {
				assert.Equal(t, editor, got.Editor)
				assert.EqualValues(t, 4200, got.Total)
				assert.Equal(t, editorRevisions, got.Revisions)
				assert.Equal(t, from, got.From)
				assert.Equal(t, to, got.To)
				assert.NoError(t, got.SyncErr)
			},
		},
		{
			name: "pays out stale figures when the wiki is unreachable",
			arrange: func(d deps) {
				d.expectSync(errWiki)
				d.editors.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(editor, nil).Once()
				d.expectSums(4200, editorRevisions, nil)
			},
			assert: func(t *testing.T, got earnings.Payslip) {
				assert.EqualValues(t, 4200, got.Total)
				assert.ErrorIs(t, got.SyncErr, errWiki, "a failed sync must be reported, not swallowed")
			},
		},
		{
			name: "reports nothing earned without failing",
			arrange: func(d deps) {
				d.expectSync(nil)
				d.editors.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(editor, nil).Once()
				d.expectSums(0, nil, nil)
			},
			assert: func(t *testing.T, got earnings.Payslip) {
				assert.Zero(t, got.Total)
				assert.Empty(t, got.Revisions)
			},
		},
		{
			name: "fails when the editor is unknown",
			arrange: func(d deps) {
				d.expectSync(nil)
				d.editors.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(entity.Editor{}, errRepo).Once()
			},
			wantErr: errRepo,
		},
		{
			name: "fails when the total cannot be read",
			arrange: func(d deps) {
				d.expectSync(nil)
				d.editors.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(editor, nil).Once()
				d.expectSums(0, nil, errRepo)
			},
			wantErr: errRepo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			tt.arrange(d)

			got, err := d.useCase().ForNickname(context.Background(), editor.Nickname, from, to)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Zero(t, got, "nothing may be returned alongside a hard error")
				return
			}

			require.NoError(t, err)
			tt.assert(t, got)
		})
	}
}

func TestUseCase_ForEditor(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(deps)
		wantErr error
		assert  func(*testing.T, earnings.Payslip)
	}{
		{
			name: "pays out what the editor earned",
			arrange: func(d deps) {
				d.expectSync(nil)
				d.editors.EXPECT().GetByID(mock.Anything, editor.EditorID).Return(editor, nil).Once()
				d.expectSums(4200, editorRevisions, nil)
			},
			assert: func(t *testing.T, got earnings.Payslip) {
				assert.Equal(t, editor, got.Editor)
				assert.EqualValues(t, 4200, got.Total)
			},
		},
		{
			name: "fails when the id is unknown",
			arrange: func(d deps) {
				d.expectSync(nil)
				d.editors.EXPECT().GetByID(mock.Anything, editor.EditorID).Return(entity.Editor{}, errRepo).Once()
			},
			wantErr: errRepo,
		},
		{
			name: "fails when the revisions cannot be listed",
			arrange: func(d deps) {
				d.expectSync(nil)
				d.editors.EXPECT().GetByID(mock.Anything, editor.EditorID).Return(editor, nil).Once()
				d.revisions.EXPECT().SumCostForEditor(mock.Anything, editor.EditorID, from, to).Return(4200, nil).Once()
				d.revisions.EXPECT().ListByEditor(mock.Anything, editor.EditorID, from, to).Return(nil, errRepo).Once()
			},
			wantErr: errRepo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			tt.arrange(d)

			got, err := d.useCase().ForEditor(context.Background(), editor.EditorID, from, to)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Zero(t, got, "nothing may be returned alongside a hard error")
				return
			}

			require.NoError(t, err)
			tt.assert(t, got)
		})
	}
}

func TestUseCase_Report(t *testing.T) {
	payroll := []entity.EditorEarnings{
		{EditorID: 1, Nickname: "alpha", Total: 100},
		{EditorID: 2, Nickname: "beta", Total: 250},
	}

	tests := []struct {
		name    string
		arrange func(deps)
		wantErr error
		assert  func(*testing.T, earnings.Report)
	}{
		{
			name: "totals every editor",
			arrange: func(d deps) {
				d.expectSync(nil)
				d.revisions.EXPECT().SumCostByEditor(mock.Anything, from, to).Return(payroll, nil).Once()
				d.corrections.EXPECT().SumByEditor(mock.Anything, from, to).Return(nil, nil).Once()
			},
			assert: func(t *testing.T, got earnings.Report) {
				assert.Equal(t, payroll, got.Editors)
				assert.EqualValues(t, 350, got.Total)
				assert.Equal(t, from, got.From)
				assert.Equal(t, to, got.To)
				assert.NoError(t, got.SyncErr)
			},
		},
		{
			name: "totals zero when nobody earned anything",
			arrange: func(d deps) {
				d.expectSync(nil)
				d.revisions.EXPECT().SumCostByEditor(mock.Anything, from, to).Return(nil, nil).Once()
				d.corrections.EXPECT().SumByEditor(mock.Anything, from, to).Return(nil, nil).Once()
			},
			assert: func(t *testing.T, got earnings.Report) {
				assert.Zero(t, got.Total)
				assert.Empty(t, got.Editors)
			},
		},
		{
			name: "reports stale totals when the wiki is unreachable",
			arrange: func(d deps) {
				d.expectSync(errWiki)
				d.revisions.EXPECT().SumCostByEditor(mock.Anything, from, to).Return(payroll, nil).Once()
				d.corrections.EXPECT().SumByEditor(mock.Anything, from, to).Return(nil, nil).Once()
			},
			assert: func(t *testing.T, got earnings.Report) {
				assert.EqualValues(t, 350, got.Total)
				assert.ErrorIs(t, got.SyncErr, errWiki)
			},
		},
		{
			name: "fails when the query fails",
			arrange: func(d deps) {
				d.expectSync(nil)
				d.revisions.EXPECT().SumCostByEditor(mock.Anything, from, to).Return(nil, errRepo).Once()
			},
			wantErr: errRepo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			tt.arrange(d)

			got, err := d.useCase().Report(context.Background(), from, to)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Zero(t, got, "nothing may be returned alongside a hard error")
				return
			}

			require.NoError(t, err)
			tt.assert(t, got)
		})
	}
}

// A payslip's total is what the edits came to plus every correction in the
// period, and the corrections themselves are carried on the payslip.
func TestUseCase_PayslipIncludesCorrections(t *testing.T) {
	corrections := []entity.PaymentCorrection{
		{CorrectionID: 1, EditorID: editor.EditorID, Amount: 1000, Description: "bonus"},
		{CorrectionID: 2, EditorID: editor.EditorID, Amount: -300, Description: "clawback"},
	}

	d := newDeps(t)
	d.expectSync(nil)
	d.editors.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(editor, nil).Once()
	d.revisions.EXPECT().SumCostForEditor(mock.Anything, editor.EditorID, from, to).Return(4200, nil).Once()
	d.revisions.EXPECT().ListByEditor(mock.Anything, editor.EditorID, from, to).Return(editorRevisions, nil).Once()
	d.corrections.EXPECT().ListByEditor(mock.Anything, editor.EditorID, from, to).Return(corrections, nil).Once()

	got, err := d.useCase().ForNickname(context.Background(), editor.Nickname, from, to)

	require.NoError(t, err)
	assert.EqualValues(t, 4200+1000-300, got.Total)
	assert.Equal(t, corrections, got.PaymentCorrections)
}

// The report folds corrections into each editor's total, and an editor whose
// only entry in the period is a correction still gets a row.
func TestUseCase_ReportMergesCorrections(t *testing.T) {
	payroll := []entity.EditorEarnings{
		{EditorID: 1, Nickname: "alpha", Total: 100},
		{EditorID: 2, Nickname: "beta", Total: 250},
	}
	corrections := []entity.EditorEarnings{
		{EditorID: 2, Nickname: "beta", Total: -50},
		{EditorID: 3, Nickname: "gamma", Total: 500},
	}

	d := newDeps(t)
	d.expectSync(nil)
	d.revisions.EXPECT().SumCostByEditor(mock.Anything, from, to).Return(payroll, nil).Once()
	d.corrections.EXPECT().SumByEditor(mock.Anything, from, to).Return(corrections, nil).Once()

	got, err := d.useCase().Report(context.Background(), from, to)

	require.NoError(t, err)
	assert.Equal(t, []entity.EditorEarnings{
		{EditorID: 1, Nickname: "alpha", Total: 100},
		{EditorID: 2, Nickname: "beta", Total: 200},
		{EditorID: 3, Nickname: "gamma", Total: 500},
	}, got.Editors)
	assert.EqualValues(t, 800, got.Total)
}

// An editor whose first edit landed since the last sync has no row yet. Sync
// first, or they are told they do not exist instead of being paid.
func TestUseCase_SyncsBeforeLookingUpTheEditor(t *testing.T) {
	d := newDeps(t)

	sync := d.syncer.EXPECT().Sync(mock.Anything).Return(nil).Once()
	d.editors.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(editor, nil).Once().NotBefore(sync)
	d.expectSums(4200, editorRevisions, nil)

	_, err := d.useCase().ForNickname(context.Background(), editor.Nickname, from, to)

	require.NoError(t, err)
}

func TestUseCase_RejectsEmptyPeriod(t *testing.T) {
	periods := []struct {
		name     string
		from, to time.Time
	}{
		{name: "to before from", from: to, to: from},
		{name: "zero width", from: from, to: from},
	}

	entryPoints := []struct {
		name string
		call func(context.Context, *earnings.UseCase, time.Time, time.Time) error
	}{
		{
			name: "ForNickname",
			call: func(ctx context.Context, uc *earnings.UseCase, from, to time.Time) error {
				_, err := uc.ForNickname(ctx, editor.Nickname, from, to)
				return err
			},
		},
		{
			name: "ForEditor",
			call: func(ctx context.Context, uc *earnings.UseCase, from, to time.Time) error {
				_, err := uc.ForEditor(ctx, editor.EditorID, from, to)
				return err
			},
		},
		{
			name: "Report",
			call: func(ctx context.Context, uc *earnings.UseCase, from, to time.Time) error {
				_, err := uc.Report(ctx, from, to)
				return err
			},
		},
	}

	for _, entry := range entryPoints {
		for _, period := range periods {
			t.Run(entry.name+"/"+period.name, func(t *testing.T) {
				// No expectations at all: any call to any collaborator fails the
				// test. A period that cannot contain anything must not reach the
				// wiki or the database.
				d := newDeps(t)

				err := entry.call(context.Background(), d.useCase(), period.from, period.to)

				require.ErrorIs(t, err, earnings.ErrEmptyPeriod)
			})
		}
	}
}

// A nil Syncer is how the caller says something else keeps storage fresh.
func TestUseCase_NilSyncerReadsWhatIsStored(t *testing.T) {
	editors := mocks.NewMockEditorReader(t)
	revisions := mocks.NewMockRevisionReader(t)
	corrections := mocks.NewMockPaymentCorrectionReader(t)

	editors.EXPECT().GetByID(mock.Anything, editor.EditorID).Return(editor, nil).Once()
	revisions.EXPECT().SumCostForEditor(mock.Anything, editor.EditorID, from, to).Return(4200, nil).Once()
	revisions.EXPECT().ListByEditor(mock.Anything, editor.EditorID, from, to).Return(editorRevisions, nil).Once()
	corrections.EXPECT().ListByEditor(mock.Anything, editor.EditorID, from, to).Return(nil, nil).Once()

	got, err := earnings.New(editors, revisions, corrections, nil).ForEditor(context.Background(), editor.EditorID, from, to)

	require.NoError(t, err)
	assert.EqualValues(t, 4200, got.Total)
	assert.NoError(t, got.SyncErr)
}

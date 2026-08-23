package revisions_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
	"github.com/the-new-day/wiki-earnings/internal/usecase/revisions"
	"github.com/the-new-day/wiki-earnings/internal/usecase/revisions/mocks"
)

var (
	errRepo = errors.New("repository is down")

	editor = entity.Editor{EditorID: 7, Nickname: "tanker"}

	revision = entity.Revision{RevID: 42, Locale: "ru", EditorID: editor.EditorID, Type: entity.MinorEdit, Cost: 1500}
)

type deps struct {
	editors   *mocks.MockEditorReader
	revisions *mocks.MockRevisionStore
}

func newDeps(t *testing.T) deps {
	t.Helper()

	return deps{
		editors:   mocks.NewMockEditorReader(t),
		revisions: mocks.NewMockRevisionStore(t),
	}
}

func (d deps) useCase() *revisions.UseCase {
	return revisions.New(d.editors, d.revisions)
}

func TestUseCase_OverridePrice(t *testing.T) {
	tests := []struct {
		name    string
		locale  string
		arrange func(deps)
		wantErr error
		assert  func(*testing.T, entity.Revision)
	}{
		{
			name:   "overrides the price when the locale is given",
			locale: "ru",
			arrange: func(d deps) {
				d.editors.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(editor, nil).Once()
				d.revisions.EXPECT().Get(mock.Anything, "ru", revision.RevID).Return(revision, nil).Once()
				d.revisions.EXPECT().OverrideCost(mock.Anything, "ru", revision.RevID, int64(5000), "admin#1").
					Return(entity.Revision{RevID: revision.RevID, Locale: "ru", EditorID: editor.EditorID, Cost: 5000}, nil).Once()
			},
			assert: func(t *testing.T, got entity.Revision) {
				assert.EqualValues(t, 5000, got.Cost)
			},
		},
		{
			name:   "resolves the locale when the editor has exactly one",
			locale: "",
			arrange: func(d deps) {
				d.editors.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(editor, nil).Once()
				d.editors.EXPECT().Locales(mock.Anything, editor.EditorID).Return([]string{"ru"}, nil).Once()
				d.revisions.EXPECT().Get(mock.Anything, "ru", revision.RevID).Return(revision, nil).Once()
				d.revisions.EXPECT().OverrideCost(mock.Anything, "ru", revision.RevID, int64(5000), "admin#1").
					Return(entity.Revision{RevID: revision.RevID, Locale: "ru", EditorID: editor.EditorID, Cost: 5000}, nil).Once()
			},
			assert: func(t *testing.T, got entity.Revision) {
				assert.EqualValues(t, 5000, got.Cost)
			},
		},
		{
			name:   "demands a locale when the editor has more than one",
			locale: "",
			arrange: func(d deps) {
				d.editors.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(editor, nil).Once()
				d.editors.EXPECT().Locales(mock.Anything, editor.EditorID).Return([]string{"ru", "ua"}, nil).Once()
			},
			wantErr: revisions.ErrLocaleRequired,
		},
		{
			name:   "fails when the editor is unknown",
			locale: "ru",
			arrange: func(d deps) {
				d.editors.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(entity.Editor{}, errRepo).Once()
			},
			wantErr: errRepo,
		},
		{
			name:   "fails when the revision cannot be found",
			locale: "ru",
			arrange: func(d deps) {
				d.editors.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(editor, nil).Once()
				d.revisions.EXPECT().Get(mock.Anything, "ru", revision.RevID).Return(entity.Revision{}, errRepo).Once()
			},
			wantErr: errRepo,
		},
		{
			name:   "rejects a revision that belongs to a different editor",
			locale: "ru",
			arrange: func(d deps) {
				other := entity.Revision{RevID: revision.RevID, Locale: "ru", EditorID: editor.EditorID + 1, Cost: 1500}
				d.editors.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(editor, nil).Once()
				d.revisions.EXPECT().Get(mock.Anything, "ru", revision.RevID).Return(other, nil).Once()
			},
			wantErr: revisions.ErrEditorMismatch,
		},
		{
			name:   "fails when the override cannot be stored",
			locale: "ru",
			arrange: func(d deps) {
				d.editors.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(editor, nil).Once()
				d.revisions.EXPECT().Get(mock.Anything, "ru", revision.RevID).Return(revision, nil).Once()
				d.revisions.EXPECT().OverrideCost(mock.Anything, "ru", revision.RevID, int64(5000), "admin#1").
					Return(entity.Revision{}, errRepo).Once()
			},
			wantErr: errRepo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			tt.arrange(d)

			got, err := d.useCase().OverridePrice(context.Background(), editor.Nickname, tt.locale, revision.RevID, 5000, "admin#1")

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

// An editor with no known locale at all is a data-integrity problem, not a
// disambiguation one -- it must fail rather than silently pick one.
func TestUseCase_OverridePrice_NoKnownLocale(t *testing.T) {
	d := newDeps(t)
	d.editors.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(editor, nil).Once()
	d.editors.EXPECT().Locales(mock.Anything, editor.EditorID).Return(nil, nil).Once()

	_, err := d.useCase().OverridePrice(context.Background(), editor.Nickname, "", revision.RevID, 5000, "admin#1")

	require.Error(t, err)
}

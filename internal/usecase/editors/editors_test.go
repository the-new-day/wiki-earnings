package editors_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
	"github.com/the-new-day/wiki-earnings/internal/storage"
	"github.com/the-new-day/wiki-earnings/internal/usecase/editors"
	"github.com/the-new-day/wiki-earnings/internal/usecase/editors/mocks"
)

var (
	errRepo = errors.New("repository is down")

	editor = entity.Editor{EditorID: 7, Nickname: "tanker"}
	paid   = entity.Editor{EditorID: 7, Nickname: "tanker", PaymentsNickname: "Tanker_2007"}
)

// repo builds the mocked repository. It is built with the *testing.T, so an
// unmet expectation fails the test at cleanup and a call nobody set up fails it
// on the spot.
func repo(t *testing.T) *mocks.MockRepository {
	t.Helper()

	return mocks.NewMockRepository(t)
}

func found(stored entity.Editor) func(*mocks.MockRepository) {
	return func(r *mocks.MockRepository) {
		r.EXPECT().FindByNickname(mock.Anything, stored.Nickname).Return(stored, nil).Once()
	}
}

func TestUseCase_SetPaymentsNickname(t *testing.T) {
	tests := []struct {
		name             string
		wikiNickname     string
		paymentsNickname string
		arrange          func(*mocks.MockRepository)
		want             string
		wantErr          error
	}{
		{
			name:             "sets the account the editor is paid on",
			wikiNickname:     editor.Nickname,
			paymentsNickname: "Tanker_2007",
			arrange: func(r *mocks.MockRepository) {
				found(editor)(r)
				r.EXPECT().SetPaymentsNickname(mock.Anything, editor.EditorID, "Tanker_2007").Return(nil).Once()
			},
			want: "Tanker_2007",
		},
		{
			name:             "an empty nickname clears it",
			wikiNickname:     paid.Nickname,
			paymentsNickname: "",
			arrange: func(r *mocks.MockRepository) {
				found(paid)(r)
				r.EXPECT().SetPaymentsNickname(mock.Anything, paid.EditorID, "").Return(nil).Once()
			},
			want: "",
		},
		{
			name:             "surrounding spaces are trimmed",
			wikiNickname:     editor.Nickname,
			paymentsNickname: "  Tanker_2007  ",
			arrange: func(r *mocks.MockRepository) {
				found(editor)(r)
				r.EXPECT().SetPaymentsNickname(mock.Anything, editor.EditorID, "Tanker_2007").Return(nil).Once()
			},
			want: "Tanker_2007",
		},
		{
			name:             "a nickname longer than the column is refused before the lookup",
			wikiNickname:     editor.Nickname,
			paymentsNickname: strings.Repeat("a", 256),
			arrange:          func(r *mocks.MockRepository) {},
			wantErr:          editors.ErrNicknameTooLong,
		},
		{
			name:             "an unknown editor is reported",
			wikiNickname:     "ghost",
			paymentsNickname: "Tanker_2007",
			arrange: func(r *mocks.MockRepository) {
				r.EXPECT().FindByNickname(mock.Anything, "ghost").Return(entity.Editor{}, storage.ErrNotFound).Once()
			},
			wantErr: storage.ErrNotFound,
		},
		{
			name:             "a failed write is reported",
			wikiNickname:     editor.Nickname,
			paymentsNickname: "Tanker_2007",
			arrange: func(r *mocks.MockRepository) {
				found(editor)(r)
				r.EXPECT().SetPaymentsNickname(mock.Anything, editor.EditorID, "Tanker_2007").Return(errRepo).Once()
			},
			wantErr: errRepo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := repo(t)
			tt.arrange(r)

			got, err := editors.New(r).SetPaymentsNickname(context.Background(), tt.wikiNickname, tt.paymentsNickname)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.PaymentsNickname)
			assert.Equal(t, tt.wikiNickname, got.Nickname)
		})
	}
}

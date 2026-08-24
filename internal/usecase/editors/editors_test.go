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

var editor = entity.Editor{EditorID: 7, Nickname: "tanker"}

func TestSetPaymentsNickname_SetsIt(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(editor, nil).Once()
	repo.EXPECT().SetPaymentsNickname(mock.Anything, editor.EditorID, "Tanker_2007").Return(nil).Once()

	got, err := editors.New(repo).SetPaymentsNickname(context.Background(), editor.Nickname, "Tanker_2007")

	require.NoError(t, err)
	assert.Equal(t, "Tanker_2007", got.PaymentsNickname)
	assert.Equal(t, editor.Nickname, got.Nickname)
}

func TestSetPaymentsNickname_EmptyClearsIt(t *testing.T) {
	stored := entity.Editor{EditorID: 7, Nickname: "tanker", PaymentsNickname: "Tanker_2007"}

	repo := mocks.NewMockRepository(t)
	repo.EXPECT().FindByNickname(mock.Anything, stored.Nickname).Return(stored, nil).Once()
	repo.EXPECT().SetPaymentsNickname(mock.Anything, stored.EditorID, "").Return(nil).Once()

	got, err := editors.New(repo).SetPaymentsNickname(context.Background(), stored.Nickname, "")

	require.NoError(t, err)
	assert.Empty(t, got.PaymentsNickname)
}

func TestSetPaymentsNickname_TrimsSpaces(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(editor, nil).Once()
	repo.EXPECT().SetPaymentsNickname(mock.Anything, editor.EditorID, "Tanker_2007").Return(nil).Once()

	got, err := editors.New(repo).SetPaymentsNickname(context.Background(), editor.Nickname, "  Tanker_2007  ")

	require.NoError(t, err)
	assert.Equal(t, "Tanker_2007", got.PaymentsNickname)
}

func TestSetPaymentsNickname_RejectsTooLongNickname(t *testing.T) {
	repo := mocks.NewMockRepository(t)

	_, err := editors.New(repo).SetPaymentsNickname(context.Background(), editor.Nickname, strings.Repeat("a", 256))

	assert.ErrorIs(t, err, editors.ErrNicknameTooLong)
}

func TestSetPaymentsNickname_PassesLookupFailureThrough(t *testing.T) {
	repo := mocks.NewMockRepository(t)
	repo.EXPECT().FindByNickname(mock.Anything, "ghost").Return(entity.Editor{}, storage.ErrNotFound).Once()

	_, err := editors.New(repo).SetPaymentsNickname(context.Background(), "ghost", "Tanker_2007")

	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestSetPaymentsNickname_PassesWriteFailureThrough(t *testing.T) {
	failure := errors.New("repository is down")

	repo := mocks.NewMockRepository(t)
	repo.EXPECT().FindByNickname(mock.Anything, editor.Nickname).Return(editor, nil).Once()
	repo.EXPECT().SetPaymentsNickname(mock.Anything, editor.EditorID, "Tanker_2007").Return(failure).Once()

	_, err := editors.New(repo).SetPaymentsNickname(context.Background(), editor.Nickname, "Tanker_2007")

	assert.ErrorIs(t, err, failure)
}

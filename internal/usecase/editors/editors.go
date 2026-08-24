// Package editors manages the editors themselves, as opposed to what they earn.
package editors

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

// maxNicknameLength is what the nickname column holds.
const maxNicknameLength = 255

var ErrNicknameTooLong = errors.New("editors: nickname too long")

type Repository interface {
	FindByNickname(ctx context.Context, nickname string) (entity.Editor, error)
	SetPaymentsNickname(ctx context.Context, editorID int64, nickname string) error
}

type UseCase struct {
	editors Repository
}

func New(editors Repository) *UseCase {
	return &UseCase{editors: editors}
}

// SetPaymentsNickname sets the account the editor found by their wiki nickname
// is paid on. An empty paymentsNickname clears it, and payments go back to the
// wiki nickname. The editor is returned as it now stands.
func (u *UseCase) SetPaymentsNickname(ctx context.Context, wikiNickname, paymentsNickname string) (entity.Editor, error) {
	paymentsNickname = strings.TrimSpace(paymentsNickname)
	if len(paymentsNickname) > maxNicknameLength {
		return entity.Editor{}, fmt.Errorf("%w: %d characters, at most %d", ErrNicknameTooLong, len(paymentsNickname), maxNicknameLength)
	}

	editor, err := u.editors.FindByNickname(ctx, wikiNickname)
	if err != nil {
		return entity.Editor{}, fmt.Errorf("editors: editor %q: %w", wikiNickname, err)
	}

	if err := u.editors.SetPaymentsNickname(ctx, editor.EditorID, paymentsNickname); err != nil {
		return entity.Editor{}, fmt.Errorf("editors: set payments nickname of %q: %w", wikiNickname, err)
	}

	editor.PaymentsNickname = paymentsNickname

	return editor, nil
}

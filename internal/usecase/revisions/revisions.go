package revisions

import (
	"context"
	"errors"
	"fmt"

	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
)

var ErrLocaleRequired = errors.New("revisions: locale required")
var ErrEditorMismatch = errors.New("revisions: revision belongs to a different editor")

type EditorReader interface {
	FindByNickname(ctx context.Context, nickname string) (entity.Editor, error)
	Locales(ctx context.Context, editorID int64) ([]string, error)
}

type RevisionStore interface {
	Get(ctx context.Context, locale string, revisionID int64) (entity.Revision, error)
	OverrideCost(ctx context.Context, locale string, revisionID int64, newCost int64, changedBy string) (entity.Revision, error)
}

type UseCase struct {
	editors   EditorReader
	revisions RevisionStore
}

func New(editors EditorReader, revisions RevisionStore) *UseCase {
	return &UseCase{
		editors:   editors,
		revisions: revisions,
	}
}

// OverridePrice sets a revision's price by hand, replacing whatever was
// computed for it -- including the flat rates for MinorEdit and ItemAddition.
// locale may be left empty only when the editor has a single wiki account:
// with more than one, the revision id alone does not say which wiki it is on.
func (u *UseCase) OverridePrice(ctx context.Context, nickname, locale string, revisionID, newCost int64, changedBy string) (entity.Revision, error) {
	editor, err := u.editors.FindByNickname(ctx, nickname)
	if err != nil {
		return entity.Revision{}, fmt.Errorf("revisions: editor %q: %w", nickname, err)
	}

	if locale == "" {
		locale, err = u.resolveLocale(ctx, editor)
		if err != nil {
			return entity.Revision{}, err
		}
	}

	revision, err := u.revisions.Get(ctx, locale, revisionID)
	if err != nil {
		return entity.Revision{}, fmt.Errorf("revisions: get %s/%d: %w", locale, revisionID, err)
	}

	if revision.EditorID != editor.EditorID {
		return entity.Revision{}, fmt.Errorf("%w: %s/%d belongs to editor %d, not %q",
			ErrEditorMismatch, locale, revisionID, revision.EditorID, nickname)
	}

	updated, err := u.revisions.OverrideCost(ctx, locale, revisionID, newCost, changedBy)
	if err != nil {
		return entity.Revision{}, fmt.Errorf("revisions: override cost %s/%d: %w", locale, revisionID, err)
	}

	return updated, nil
}

// resolveLocale picks the one locale an editor is known to edit under, or
// fails asking for one when there is more than one to choose from.
func (u *UseCase) resolveLocale(ctx context.Context, editor entity.Editor) (string, error) {
	locales, err := u.editors.Locales(ctx, editor.EditorID)
	if err != nil {
		return "", fmt.Errorf("revisions: locales for editor %d: %w", editor.EditorID, err)
	}

	switch len(locales) {
	case 0:
		return "", fmt.Errorf("revisions: editor %d has no known locale", editor.EditorID)
	case 1:
		return locales[0], nil
	default:
		return "", fmt.Errorf("%w: editor %q has accounts in %v", ErrLocaleRequired, editor.Nickname, locales)
	}
}

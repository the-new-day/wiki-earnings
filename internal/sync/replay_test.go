package sync_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

const failedRevID = 11

// failedRevision is a parked revision with the given number of attempts already
// behind it.
func failedRevision(attempts int) entity.FailedRevision {
	return entity.FailedRevision{
		RevID:      failedRevID,
		Locale:     locale,
		PageID:     100,
		PageTitle:  "Tank",
		WikiUserID: wikiUserID,
		Nickname:   "tanker",
		Comment:    "(AE) add a section",
		Type:       entity.ArticleEdit,
		EditedAt:   firstEdit,
		Attempts:   attempts,
		Status:     entity.FailedPending,
	}
}

// editIsUnfetchable is the reason a revision typically sits in the dead letter:
// the wiki would not hand over the before/after it has to be priced from.
func (d *deps) editIsUnfetchable() {
	d.editorIsKnown()
	d.wiki.EXPECT().FetchEdit(mock.Anything, mock.Anything, mock.Anything, locale).
		Return(entity.Edit{}, errWiki)
}

func reasonMentions(want string) any {
	return mock.MatchedBy(func(reason string) bool {
		return strings.Contains(reason, want)
	})
}

func TestService_Replay(t *testing.T) {
	tests := []struct {
		name    string
		failed  entity.FailedRevision
		arrange func(*deps)
	}{
		{
			name:   "a revision that finally goes through leaves the dead letter",
			failed: failedRevision(1),
			arrange: func(d *deps) {
				d.editorIsKnown()
				d.editIsFetchable()
				d.pricesAt(9000)
				d.revisions.EXPECT().Upsert(mock.Anything, mock.Anything).Return(nil)
				d.dead.EXPECT().Resolve(mock.Anything, locale, int64(failedRevID)).Return(nil)
			},
		},
		{
			name:   "a revision with attempts left waits for the next pass",
			failed: failedRevision(1),
			arrange: func(d *deps) {
				d.editIsUnfetchable()
				d.dead.EXPECT().
					Fail(mock.Anything, locale, int64(failedRevID), reasonMentions(errWiki.Error())).
					Return(nil)
			},
		},
		{
			name: "a revision out of attempts is retired",
			// One short of MaxAttempts: this pass is the one that exhausts it.
			failed: failedRevision(2),
			arrange: func(d *deps) {
				d.editIsUnfetchable()
				d.dead.EXPECT().
					Retire(mock.Anything, locale, int64(failedRevID), reasonMentions(errWiki.Error())).
					Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t)
			d.dead.EXPECT().ListPending(mock.Anything, d.cfg.ReplayBatchSize).
				Return([]entity.FailedRevision{tt.failed}, nil)
			tt.arrange(d)

			// A revision failing again is an expected outcome of a replay, not
			// a failure of the replay itself.
			require.NoError(t, d.service().Replay(context.Background()))
		})
	}
}

func TestService_ReplayTrustsTheStoredClassification(t *testing.T) {
	failed := failedRevision(1)
	// The dead letter already knows what kind of work this was, and the comment
	// it was classified from may since have been edited away.
	failed.Type = entity.TranslatedArticle
	failed.Comment = "comment without a tag"

	d := newDeps(t)
	d.dead.EXPECT().ListPending(mock.Anything, d.cfg.ReplayBatchSize).
		Return([]entity.FailedRevision{failed}, nil)
	d.editorIsKnown()
	d.editIsFetchable()
	d.pricesAt(9000)
	stored := d.stores()
	d.dead.EXPECT().Resolve(mock.Anything, locale, int64(failedRevID)).Return(nil)

	require.NoError(t, d.service().Replay(context.Background()))

	rev := stored.last(t)
	assert.Equal(t, entity.TranslatedArticle, rev.Type)
	assert.Equal(t, int64(9000), rev.Cost)
	assert.Equal(t, editor.EditorID, rev.EditorID)
	assert.Equal(t, failed.EditedAt, rev.EditedAt, "a replay pays for when the edit happened, not when it was retried")
}

func TestService_ReplayFailsWhenTheDeadLetterCannotBeRead(t *testing.T) {
	d := newDeps(t)
	d.dead.EXPECT().ListPending(mock.Anything, d.cfg.ReplayBatchSize).Return(nil, errStore)

	require.ErrorIs(t, d.service().Replay(context.Background()), errStore)
}

package sync

import (
	"strings"
	"time"

	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
	"github.com/the-new-day/protanki-wiki-admin/internal/mediawiki"
)

// change is one edit on its way through the pipeline. It comes either from the
// wiki's recent changes or from the dead letter, and the work is the same
// either way, so both are narrowed to this.
type change struct {
	RevID     int64
	PageID    int64
	Title     string
	UserID    int64
	User      string
	Comment   string
	Timestamp time.Time
}

func changeFromRecent(c mediawiki.RecentChange) change {
	return change{
		RevID:     c.RevID,
		PageID:    c.PageID,
		Title:     c.Title,
		UserID:    c.UserID,
		User:      c.User,
		Comment:   c.Comment,
		Timestamp: c.Timestamp,
	}
}

func changeFromFailed(f entity.FailedRevision) change {
	return change{
		RevID:     f.RevID,
		PageID:    f.PageID,
		Title:     f.PageTitle,
		UserID:    f.WikiUserID,
		User:      f.Nickname,
		Comment:   f.Comment,
		Timestamp: f.EditedAt,
	}
}

// taggedTypes maps the tags editors put in their edit summaries to the kind of
// work being claimed. Order is the tie-break for a comment carrying more than
// one tag: the kinds that price an article outright come before the ones that
// price a difference, because an article that is new or translated has no
// meaningful "before" to diff against.
var taggedTypes = []struct {
	tag  string
	kind entity.RevisionType
}{
	{mediawiki.NewArticleEditTag, entity.NewArticle},
	{mediawiki.TranslatedArticleEditTag, entity.TranslatedArticle},
	{mediawiki.RefactoredArticleEditTag, entity.RefactoredArticle},
	{mediawiki.ItemAdditionEditTag, entity.ItemAddition},
	{mediawiki.ArticleEditTag, entity.ArticleEdit},
	{mediawiki.MinorEditTag, entity.MinorEdit},
}

// classify reads what kind of work an edit summary claims. An untagged comment
// is not an error: the tag is how an editor asks to be paid, and plenty of
// edits never ask.
func classify(comment string) (entity.RevisionType, bool) {
	for _, t := range taggedTypes {
		if strings.Contains(comment, t.tag) {
			return t.kind, true
		}
	}

	return 0, false
}

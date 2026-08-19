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

func changeFromRecent(c entity.RecentChange) change {
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

var taggedTypes = []struct {
	tag  string
	kind entity.RevisionType
}{
	// Order matters: it's the tie-break for a comment carrying more than one tag
	{mediawiki.NewArticleEditTag, entity.NewArticle},
	{mediawiki.TranslatedArticleEditTag, entity.TranslatedArticle},
	{mediawiki.RefactoredArticleEditTag, entity.RefactoredArticle},
	{mediawiki.ItemAdditionEditTag, entity.ItemAddition},
	{mediawiki.ArticleEditTag, entity.ArticleEdit},
	{mediawiki.MinorEditTag, entity.MinorEdit},
}

// classify reads what kind of work an edit summary claims (if any).
func classify(comment string) (entity.RevisionType, bool) {
	for _, t := range taggedTypes {
		if strings.Contains(comment, t.tag) {
			return t.kind, true
		}
	}

	return 0, false
}

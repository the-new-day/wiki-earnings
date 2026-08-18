package entity

import "time"

type RevisionType int

const (
	MinorEdit         RevisionType = 1
	ItemAddition      RevisionType = 2
	ArticleEdit       RevisionType = 3
	RefactoredArticle RevisionType = 4
	NewArticle        RevisionType = 5
	TranslatedArticle RevisionType = 6
)

type Revision struct {
	RevID      int64        `db:"revision_id"`
	Locale     string       `db:"locale"`
	EditorID   int64        `db:"editor_id"`
	PageID     int64        `db:"page_id"`
	PageTitle  string       `db:"page_title"`
	Type       RevisionType `db:"type"`
	Comment    string       `db:"comment"`
	Cost       int64        `db:"cost"`
	EditedAt   time.Time    `db:"edited_at"`
	ComputedAt time.Time    `db:"computed_at"`
}

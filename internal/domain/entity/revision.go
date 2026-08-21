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
	RevID          int64
	Locale         string
	EditorID       int64
	PageID         int64
	PageTitle      string
	Type           RevisionType
	Comment        string
	Cost           int64
	EditedAt       time.Time
	ComputedAt     time.Time
	CostOverridden bool
}

func (r RevisionType) IsMinor() bool {
	return r == MinorEdit || r == ItemAddition
}

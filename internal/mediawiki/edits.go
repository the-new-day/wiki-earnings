package mediawiki

import "errors"

var (
	ErrPageNotFound     = errors.New("page not found")
	ErrNoRevisions      = errors.New("no revisions found")
	ErrNoReferencePoint = errors.New("no reference point found")
)

const wikiUrl = "https://wiki.pro-tanki.online/"

const (
	MinorEditTag             = "(ME)"
	ItemAdditionEditTag      = "(IA)"
	ArticleEditTag           = "(AE)"
	RefactoredArticleEditTag = "(RA)"
	TranslatedArticleEditTag = "(TA)"
	NewArticleEditTag        = "(NA)"
)

func possibleTags() []string {
	return []string{
		MinorEditTag, ItemAdditionEditTag, ArticleEditTag,
		RefactoredArticleEditTag, TranslatedArticleEditTag, NewArticleEditTag,
	}
}

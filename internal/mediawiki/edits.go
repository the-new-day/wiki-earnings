package mediawiki

import "errors"

var (
	ErrPageNotFound             = errors.New("page not found")
	ErrNoRevisions              = errors.New("no revisions found")
	ErrNoReferencePoint         = errors.New("no reference point found")
	ErrTitleAndRevisionMismatch = errors.New("revision from a different page") // TODO: use for edit fetching (not used yet)
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
